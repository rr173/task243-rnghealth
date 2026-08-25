package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"task243-rnghealth/internal/httpapi"
	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "rnghealth.db", "SQLite 数据库路径")
	smoke := flag.Bool("smoke-test", false, "运行自检（不启动长驻服务），验证持久化与重启恢复后退出")
	flag.Parse()

	if *smoke {
		if err := runSmoke(*dbPath); err != nil {
			fmt.Fprintln(os.Stderr, "SMOKE FAIL:", err)
			os.Exit(1)
		}
		fmt.Println("SMOKE OK")
		os.Exit(0)
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open db:", err)
		os.Exit(1)
	}
	defer db.Close()

	svc := service.New(db)
	srv := &http.Server{
		Addr:    *addr,
		Handler: httpapi.New(svc).Routes(),
	}
	fmt.Printf("rnghealth listening on %s (db=%s)\n", *addr, *dbPath)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

// runSmoke 真实创建熵源与窗口、执行健康测试、触发持续异常关联、记录重启边界、
// 发布诊断快照；关闭并重新打开数据库验证持久化与重启恢复，最后以 0 退出。
func runSmoke(dbPath string) error {
	tmp := dbPath
	if tmp == "rnghealth.db" {
		tmp = filepath.Join(os.TempDir(), fmt.Sprintf("rnghealth-smoke-%d.db", os.Getpid()))
	}
	for _, suf := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(tmp + suf)
	}

	db, err := store.Open(tmp)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	svc := service.New(db)
	now := time.Now()

	srcID, err := svc.RegisterSource("smoke-trng", "dev-smoke-0", now)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	// 健康窗口批次（高熵样本，应通过健康测试）。
	const goodN = 10
	for i := 1; i <= goodN; i++ {
		if _, err := svc.IngestWindow(srcID, int64(i), goodSample(uint64(i)), now); err != nil {
			return fmt.Errorf("ingest good %d: %w", i, err)
		}
	}
	src, err := svc.GetSource(srcID)
	if err != nil {
		return err
	}
	if src.State != model.SourceStateObserving {
		return fmt.Errorf("expected observing after first window, got %s", src.State)
	}

	// 异常窗口批次（停滞样本：全 0），应触发重复/统计异常并持续关联降级。
	const badN = 5
	base := goodN
	for i := 1; i <= badN; i++ {
		if _, err := svc.IngestWindow(srcID, int64(base+i), badSample(), now); err != nil {
			return fmt.Errorf("ingest bad %d: %w", i, err)
		}
	}

	// 记录设备重启边界（未提供恢复基线）。
	if _, err := svc.RecordRestart(srcID, int64(base+badN), 0, now); err != nil {
		return fmt.Errorf("restart: %w", err)
	}

	// 发布诊断快照。
	snap, err := svc.DraftSnapshot(srcID, now)
	if err != nil {
		return fmt.Errorf("draft snapshot: %w", err)
	}
	pub, err := svc.PublishSnapshot(snap.ID, now)
	if err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	if pub.State != model.SnapshotStatePublished {
		return fmt.Errorf("snapshot not published: %s", pub.State)
	}

	// 验证降级已发生。
	src, err = svc.GetSource(srcID)
	if err != nil {
		return err
	}
	if src.State != model.SourceStateDegraded {
		return fmt.Errorf("expected degraded after persistent anomaly, got %s", src.State)
	}
	evs, err := svc.ListEvents(srcID, model.EventStatePersistent, 10, 0)
	if err != nil {
		return err
	}
	if len(evs) == 0 {
		return fmt.Errorf("expected at least one persistent event")
	}

	// 关闭并重新打开数据库，验证持久化与重启恢复。
	if err := db.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	db2, err := store.Open(tmp)
	if err != nil {
		return fmt.Errorf("reopen: %w", err)
	}
	defer db2.Close()
	svc2 := service.New(db2)

	src2, err := svc2.GetSource(srcID)
	if err != nil {
		return err
	}
	if src2.State != model.SourceStateDegraded {
		return fmt.Errorf("after reopen: state %s, want degraded", src2.State)
	}
	wins, err := svc2.ListWindows(srcID, "", 100, 0)
	if err != nil {
		return err
	}
	if len(wins) != goodN+badN {
		return fmt.Errorf("after reopen: window count %d, want %d", len(wins), goodN+badN)
	}
	snap2, err := svc2.GetSnapshot(snap.ID)
	if err != nil {
		return err
	}
	if snap2.State != model.SnapshotStatePublished {
		return fmt.Errorf("after reopen: snapshot state %s", snap2.State)
	}

	// 自检全链路。
	if !svc2.SelfCheck().OK {
		return fmt.Errorf("selfcheck reported failure after reopen")
	}
	return nil
}

// goodSample 由 SHA-256 链式派生高熵字节样本（模拟健康随机数）。
func goodSample(seed uint64) []byte {
	const n = 256
	out := make([]byte, n)
	sum := sha256.Sum256([]byte(fmt.Sprintf("rng-seed-%d", seed)))
	copy(out[:32], sum[:])
	for i := 32; i < n; i += 32 {
		sum = sha256.Sum256(out[i-32 : i])
		copy(out[i:i+32], sum[:])
	}
	return out
}

// badSample 返回全 0 停滞样本（模拟熵源退化/重播）。
func badSample() []byte {
	return make([]byte, 256)
}
