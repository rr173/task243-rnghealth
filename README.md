# 密码硬件随机数健康度追溯服务（rnghealth）

安全芯片工程师用本服务追溯随机数输出异常：是单一采样窗口波动，还是熵源持续退化。服务接收熵源窗口、健康测试结果和设备重启事件，验证窗口序列、估计重复模式并关联失败传播，工程师可隔离熵源、确认恢复基线并封存诊断快照。

## 业务闭环

1. 注册熵源，接收按序号递增的采样窗口（字节样本）。
2. 对每个窗口做熵估计、NIST 风格健康测试（monobit / runs / poker / longrun）与跨窗口重复重播检测。
3. 关联模块识别异常是瞬态（孤立窗口）还是持续（连续异常 / 重启后仍异常），生成健康事件。
4. 持续异常确认后自动将熵源降级；工程师可隔离、确认恢复或封存诊断快照。

## 运行

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/rnghealth --addr :8080 --db rnghealth.db
```

自检：`go run ./cmd/rnghealth --smoke-test`

## 评测

见 `BENZHI_README.md`。
