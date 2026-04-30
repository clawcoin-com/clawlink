# 多开 clcli daemon — 内部容量与调度指南

> **INTERNAL USE ONLY** — 基于 RTX 5070 12GB + Ollama 0.21.2 + gemma4:e4b 的实测数据。
> 数据采集时间 2026-04-25，硬件/软件升级后建议重测。

---

## TL;DR

- **GPU 硬上限**：12 GB VRAM 下，gemma4:e4b 最多支持 **5 个 daemon 同时活跃**，第 6 个会 OOM
- **时间不是瓶颈**：5 个并发请求最坏 13.8s，远小于 daemon 默认 60s interval，所以不冲突
- **真正该错峰的原因**：让响应延迟稳定 + 给宿主机留显存 buffer
- **推荐配置**：N ≤ 4 直接同时启动；N ≥ 5 用 staggered launch script

---

## 1. 实测硬件基线 (RTX 5070 12 GB)

### 1.1 单一请求 (热机, gemma4:e4b 已加载)

| 负载类型 | Prompt tokens | Output tokens | 耗时 |
|---|---|---|---|
| Health-check (preflight) | 18 in | 1 out | 430 ms |
| Real daemon brain (system + user prompt) | ~600 in | 300 out | 3.2 s |

### 1.2 并发负载 (真实 daemon brain, 600 in / 300 out)

| 并发数 | 最早完成 | 最晚完成 | 总占用 GPU 时间 | GPU 显存占用 |
|---|---|---|---|---|
| 1 (串行) | 3.2 s | 3.2 s | 3.2 s | 6.5 GB |
| 3 同时 fire | 3.2 s | **8.5 s** | 8.5 s | ~9 GB |
| 5 同时 fire | 3.3 s | **13.8 s** | 13.8 s | **11.5 GB**（接近 12GB 极限）|
| 5 串行 (对照) | — | — | 16 s | 6.5 GB |

**核心洞察**：Ollama 的 "并发" 不是真 GPU 并行，而是请求 **batch 进同一个推理 step**。
每多一个并发请求，每个 forward pass 多算一份 attention/FFN，整体延迟近似线性增加。
所以 5 并发 (13.8s) ≈ 5 串行 (16s) - 14%。

### 1.3 显存使用模式

```
gemma4:e4b 模型权重:        5.5 GB
1 个请求的 KV cache:         ~1.3 GB  (n_ctx=4096)
基线 1 并发 = 5.5 + 1.3   =  6.8 GB
3 并发     = 5.5 + 3×1.3  =  9.4 GB
5 并发     = 5.5 + 5×1.3  = 12.0 GB ← 触顶
```

第 6 个并发请求 → **`out of memory` 错误**，Ollama 返回 500，clcli daemon 走 `error` 分支记 audit log，下个 cycle 重试。不会崩。

---

## 2. 多开调度策略

### 2.1 模式 A：完全并发（N ≤ 4）

每个 daemon 独立启动，不协调。所有 daemon 在 60s interval 边界各自 fire。

```powershell
# 同时启动 3 个 agent，新窗口分别跑
Start-Process clcli -ArgumentList "--profile alpha agent run"
Start-Process clcli -ArgumentList "--profile bravo agent run"
Start-Process clcli -ArgumentList "--profile charlie agent run"
```

**特性**：
- 启动后 ~1 秒内全部 fire 第一次 LLM 调用 → 并发 3 → 最坏 8.5s 完成
- 剩 51s 闲置直到下一个 60s 周期
- **不会撞下一个 cycle**（8.5s ≪ 60s，留 51s buffer）

**适用**：N ∈ {1,2,3,4}, 不在乎 1.5x 延迟波动

### 2.2 模式 B：错峰启动（N = 5-6）

每个 daemon 启动时刻错开 12-15s，让它们在每分钟内的 LLM 调用时刻自然分散。

```powershell
# stagger-launch.ps1 — 5 个 agent 错峰 12s
$profiles = "alpha","bravo","charlie","delta","echo"
foreach ($i in 0..($profiles.Count - 1)) {
    if ($i -gt 0) { Start-Sleep -Seconds 12 }
    $name = $profiles[$i]
    Start-Process clcli -ArgumentList "--profile $name agent run" -WindowStyle Hidden
    Write-Output "[$([DateTime]::Now.ToString('HH:mm:ss'))] launched $name"
}
```

启动后稳态调度图：
```
t=0       12       24       36       48       60       72       ...
α  ███───────────────────────────────────████───────────────────
β  ────████──────────────────────────────────███────────────────
γ  ─────────────███───────────────────────────────███───────────
δ  ──────────────────────████──────────────────────────███──────
ε  ───────────────────────────────███───────────────────────────████
                                                    ↑
                                      α 的下一次刚好 60s 后,
                                      和 ε 自然错开
```

**特性**：
- 每时刻最多 1-2 个 daemon 在算
- 显存稳定在 6.5-7.5 GB
- 决策延迟稳定在 3-4 s

**适用**：N ∈ {5,6}

### 2.3 模式 C：长 interval（N ≥ 7）

把 daemon `--interval` 调到 90s 或 120s。结合错峰启动，相当于把同一时刻的活跃 daemon 数压回 ≤ 4。

```powershell
# 7 个 agent, 120s interval, 错峰 17s
foreach ($i in 0..6) {
    if ($i -gt 0) { Start-Sleep -Seconds 17 }
    Start-Process clcli -ArgumentList "--profile agent-$i agent run --interval 120s"
}
```

**适用**：N ≥ 7，或者你想给宿主机留更多显存做别的事

### 2.4 模式 D：本地 + 云混合（N ≥ 8 或想要顶级 brain）

部分 daemon 用 gemma4 (LiteLLM gateway 的 `gemma4`)，重要的几个用 claude-haiku 或 DeepSeek-V3。改 `CLCLI_LLM_MODEL` 即可，不动其他东西。

```powershell
# 8 个 agent: 5 用 gemma4 本地, 3 用 claude-haiku 云
foreach ($i in 0..4) {
    $env:CLCLI_LLM_MODEL = "gemma4"
    Start-Process clcli -ArgumentList "--profile local-$i agent run"
    Start-Sleep -Seconds 12
}
foreach ($i in 0..2) {
    $env:CLCLI_LLM_MODEL = "claude-haiku"
    Start-Process clcli -ArgumentList "--profile premium-$i agent run"
    Start-Sleep -Seconds 5   # 云模型 2s 决策, 错峰可以更短
}
```

**适用**：production，多角色 agent

---

## 3. 实操：staggered-launch.ps1 完整脚本

放在 `D:\docker\agents\stagger-launch.ps1`：

```powershell
<#
.SYNOPSIS
    Launch N clcli daemons with staggered start times to spread LLM load.
.PARAMETER Count
    Number of daemons to launch.
.PARAMETER Stagger
    Seconds between launches (default: 60/Count to spread evenly within 1 minute).
.PARAMETER Model
    LLM model name (passed via CLCLI_LLM_MODEL).
.PARAMETER Interval
    daemon --interval flag (default 60s).
.EXAMPLE
    .\stagger-launch.ps1 -Count 5 -Model gemma4
.EXAMPLE
    .\stagger-launch.ps1 -Count 7 -Model gemma4 -Interval 90s
#>
param(
    [int]   $Count    = 3,
    [int]   $Stagger  = 0,
    [string]$Model    = "gemma4",
    [string]$Interval = "60s"
)

if ($Stagger -eq 0) {
    # Default: spread evenly across one interval period.
    $intervalSec = [int]([TimeSpan]::Parse($Interval -replace 's$').TotalSeconds)
    if (-not $intervalSec) { $intervalSec = 60 }
    $Stagger = [Math]::Max(5, [int]($intervalSec / $Count))
}

$env:CLCLI_LLM_MODEL    = $Model
$env:CLCLI_API_BASE_URL = "http://127.0.0.1:8080/api/v1"
# CLCLI_LLM_API_KEY must already be in env.

if (-not $env:CLCLI_LLM_API_KEY) {
    Write-Error "CLCLI_LLM_API_KEY not set. Aborting."
    exit 1
}

$clcli = "D:\work\clawcoin-com\clcli\build\clcli.exe"

Write-Host "Launching $Count daemons (model=$Model interval=$Interval stagger=${Stagger}s)..."
for ($i = 0; $i -lt $Count; $i++) {
    if ($i -gt 0) { Start-Sleep -Seconds $Stagger }
    $name = "agent-$i"
    Start-Process -FilePath $clcli `
        -ArgumentList "--profile",$name,"agent","run","--interval",$Interval `
        -WindowStyle Hidden
    Write-Host ("[{0}] launched {1}" -f (Get-Date -Format 'HH:mm:ss'), $name)
}

Write-Host ""
Write-Host "Done. Tail logs with:"
Write-Host "  Get-Content `$env:USERPROFILE\.clawlink\profiles\agent-0\daemon.log.jsonl -Wait"
```

---

## 4. 预热（消除首次冷启动 50s）

模型从磁盘加载到 GPU 一次大约 50 秒。如果 daemon 启动 → 第一次 LLM 调用之间间隔很久，
模型可能已经被 unload。建议在启动 daemon 之前预热：

```powershell
# warmup-ollama.ps1
$body = '{"model":"gemma4:e4b","prompt":" ","stream":false,"keep_alive":"30m"}'
Write-Host "Warming gemma4:e4b into VRAM..."
$sw = [System.Diagnostics.Stopwatch]::StartNew()
curl.exe -s -X POST "http://127.0.0.1:11434/api/generate" -H "Content-Type: application/json" -d $body | Out-Null
$sw.Stop()
Write-Host "Warm-up done in $($sw.ElapsedMilliseconds) ms (subsequent loads should be < 1s)."
```

放在 stagger-launch.ps1 第一行（在循环之前）。

---

## 5. 显存监控

实时盯：

```powershell
nvidia-smi -l 5 --query-gpu=memory.used,memory.free,utilization.gpu --format=csv
```

显存超过 11 GB 时该警觉。超过 11.8 GB 下次请求大概率 OOM。

**应急处理**：

```powershell
# 立刻关掉一两个 daemon
Get-Process clcli | Sort-Object StartTime -Descending | Select-Object -First 2 | Stop-Process

# 或者强制 unload 模型，下次请求重新加载（5s）
docker exec ollama ollama stop gemma4:e4b
```

---

## 6. 故障表

| 症状 | 原因 | 解法 |
|---|---|---|
| daemon log 显示 `transport: ollama 500: out of memory` | 同时活跃 daemon 数 × KV cache 超 12GB | 减 daemon 数 或 改用 `gemma4:e2b` (7.2GB)|
| 单 daemon 决策耗时从 3s 升到 30s | 同卡上有人开了游戏 / OBS / 其他 GPU 程序抢资源 | 关掉抢资源的程序，或迁移那些 daemon 到云模型 |
| daemon 启动几小时后突然全部 401 | API key 被某处 rotate 了（很少见） | 重新 `clcli auth login` 每个 profile |
| Ollama 容器被 Docker Desktop 杀掉 | WSL2 内存不够 | 提升 WSL2 内存上限 (`%USERPROFILE%\.wslconfig`)|
| 5 个 daemon 跑了 1 小时后 GPU 显存"漂移"到 11.8 GB | Ollama KV cache 累积没释放 | 临时 `docker exec ollama ollama stop gemma4:e4b` 释放，下次请求重 load|

---

## 7. 何时切换到云端模型

继续追加 daemon 数量 **不应该** 通过升级硬件解决——超出某个临界点反而走云更经济。

| Daemon 数 | 推荐配置 | 月成本估算 |
|---|---|---|
| 1-4 | gemma4:e4b 本地，模式 A | $0 |
| 5-6 | gemma4:e4b 本地，模式 B 错峰 | $0 |
| 7-10 | gemma4:e4b 错峰 + 长 interval (模式 C) 或 部分迁云 (模式 D) | $0 - $30 |
| 11-30 | 全部走 LiteLLM 路由到 claude-haiku 或 DeepSeek-V3 | $30 - $200 |
| 30+ | 自建 vLLM 多卡 | 视卡而定 |

云端 brain 的另一个优点：**响应 1-3s** vs 本地 3-15s，用户体验明显好。但开销上面讲的。

---

## 8. 测试清单（多开上线前必跑）

```powershell
# 1. 单 daemon 跑通
clcli --profile test-1 agent run --once --dry-run --verbose
# 期望: preflight OK, 一个 cycle 完成

# 2. 5 并发健康检查
.\stagger-launch.ps1 -Count 5 -Model gemma4
Start-Sleep -Seconds 90
Get-Content $env:USERPROFILE\.clawlink\profiles\agent-0\daemon.log.jsonl -Tail 5
nvidia-smi --query-gpu=memory.used --format=csv

# 期望: 每个 profile 至少 1 条 audit 记录, GPU 显存 < 11.5GB

# 3. 关闭后清理
Get-Process clcli | Stop-Process
docker exec ollama ollama ps   # 确认模型卸载或保留按预期
```

---

## 9. 关键决策记录

| 选项 | 我们的决策 | 原因 |
|---|---|---|
| 用 vLLM 替代 Ollama | **不** | vLLM 在 12GB 单卡上对 gemma4 这种小模型优势 < 20%，而要 WSL2 + 复杂配置 |
| Ollama `OLLAMA_NUM_PARALLEL` 显式设置 | **保持默认** | Ollama 0.21+ 会根据显存自动选；显式设大反而提前 OOM |
| daemon 加 `--phase-offset` flag | **不加** | 用 PowerShell 启动脚本 sleep 已经够用，加 flag 增加表面积 |
| daemon 启动时自动预热 LLM | **不加** | 跨进程预热复杂；用脚本提前 warmup-ollama.ps1 更可控 |

---

> Last updated: 2026-04-25
>
> Hardware baseline: RTX 5070 12 GB / Ollama 0.21.2 / gemma4:e4b / Windows 11 +
> Docker Desktop / WSL2
