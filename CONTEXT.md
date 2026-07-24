# Domain Context

## Progress Display

### WorkerProgress
单个 worker 当前下载文件的进度。
- `url` — 下载的 URL
- `downloaded` — 已下载字节数
- `total` — 文件总字节数（如未知则为空）

### GlobalProgress
所有 worker 的整体完成计数。
- `completed` — 已完成的 worker 数
- `total` — 所有 URL 总数
- 显示为 `[completed/total]`，位于进度区域最下方

### Rendering
- ANSI escape codes 原地刷新 stderr
- 固定间隔 ticker（~100ms）批量渲染

### Completion
- 全部完成后擦除整个进度区域
- 保留最终行 `[total/total] done`

### Layout
- rows = `min(workers, 16)` 向上取偶 ÷ 2
- max 8 rows (16 workers)
- 每行 split 左右，各一个 WorkerProgress
- url 超过半行宽度截断加 `...`
- workers > 16 只取前 16
