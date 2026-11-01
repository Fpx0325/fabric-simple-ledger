# 验证记录

2026-11-01，Windows / Go 1.24.0。

已执行并通过：
- 根模块 `go test ./...`、`go vet ./...`、`go build ./...`。
- chaincode 模块 `go test ./...`、`go vet ./...`、`go build ./...`。
- `node --check web/app.js`。
- `gofmt` 格式化；UI 静态审计结果见 `ui-audit.json`。

测试覆盖：链码构造、创建与查询、重复编号拒绝、非法参数、记录不存在、Org2 只读、未知组织拒绝；HTTP 令牌鉴权、输入验证在依赖调用之前执行。

尚未验证：
- 当前环境未安装 Docker，无法进行真实 Fabric + MySQL 部署与端到端测试。
- 内嵌浏览器访问本地预览端口超时，未完成视觉和浏览器交互验收。
- 当前环境没有 GCC，未运行 race；Linux GitHub Actions 已配置 `go test -race ./...`，推送后需检查实际运行结果。
- 未把 MySQL 故障及链上超时模拟测试结果当作已完成。README 提供真实环境验收步骤。

依赖已生成 go.sum。官方 Go 代理连接超时，本次通过 goproxy.cn 下载公开模块，保留默认 Go checksum 校验；镜像设置仅作用于下载命令，不写入项目配置。

发布状态：目前为本地工程，GitHub 身份登录与仓库发布需要由账号本人完成登录后继续。
