# Lounge 插件 SDK

用户中心的**运行时插件**：管理员上传（或从插件市场下载）一个签名插件包、确认权限后启用，不需要重新构建服务器。架构与运维说明见根目录 `docs/PLUGINS.md`。

| 文档 | 内容 |
| --- | --- |
| [MANIFEST.md](MANIFEST.md) | `plugin.json` 全部字段、能力与数据权限、扩展点与界面插槽、设置结构 |
| [ABI.md](ABI.md) | WebAssembly 宿主 ABI、宿主函数、处理函数的输入输出、外部应用协议、页面桥接、插件包与签名、市场索引 |

## 三种插件，一种清单

| 类型 | 适合 | 代码在哪运行 |
| --- | --- | --- |
| `declarative` 声明式 | 设置项、自定义用户字段、菜单链接、把事件推送到你的 URL | 没有代码 |
| `wasm` 沙箱 | 事件处理、令牌字段、GM 操作、定时任务、插件页面的后端 | 用户中心进程内的 WebAssembly 沙箱（wazero），按次限制内存 / 时间 / 宿主调用 |
| `external` 外部应用 | 已有服务、需要数据库或长任务的集成 | 你自己的 HTTPS 服务：收签名事件，用插件令牌回调 |

插件能看到什么、能做什么，全部写在清单里（`permissions` 数据权限、`capabilities` 能力）；管理员安装前逐条确认，未授予的能力一律拒绝并写入审计日志。

## 五分钟跑通示例（Go → WebAssembly）

需要 Go 1.24 或更新版本（`//go:wasmexport`）。

```bash
cd SDK/plugins/examples/hello
./build.sh            # Windows: .\build.ps1
```

脚本做三件事：`GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .`；第一次运行时生成开发密钥 `dev.key` / `dev.pub`；用 `tools/loungeplugin` 打包并签名为 `dist/com.example.hello-1.0.0.zip`。

然后在后台「系统 → 插件市场」：

1. 「设置 → 受信任的发布者公钥」添加 `dev.pub` 的内容（只有超级管理员能改信任设置）；
2. 「已安装 → 上传插件包」选择 zip，核对确认页上的签名、数据权限与能力，同意并安装；
3. 插件详情里填写设置，再打开开关启用。

示例会统计每个用户的登录次数（事件 `user.login`）、把次数写进令牌字段（`token.claims`）、在服务器 GM 面板里提供 GM 操作「问候玩家」、每小时写一行日志，并在用户中心「应用与插件」里显示一个页面。

## 目录

```
plugins/
├── guest/                  # Go 访客库: Input / Output / Log / Call (GOOS=wasip1)
├── examples/hello/         # Go → WebAssembly 示例 (事件、过滤、GM、任务、页面 + 桥接)
├── examples/external-app/  # 外部应用示例 (标准库, 校验签名、用令牌回调)
├── schema/                 # 标准 1.1 的 JSON Schema (plugin.json / index.json) 与校验器
└── tools/loungeplugin/     # keygen / pack / verify / index / lint (只用标准库)
```

## 打包与签名

```bash
go run ./tools/loungeplugin keygen -out publisher          # publisher.key 保密, publisher.pub 给管理员
go run ./tools/loungeplugin pack -dir my-plugin -key publisher.key -out dist
go run ./tools/loungeplugin verify -zip dist/<id>-<version>.zip -pub publisher.pub
go run ./tools/loungeplugin lint -dir my-plugin                              # 按标准 1.1 检查
go run ./tools/loungeplugin index -dir dist -name "我的插件" > index.json   # 自建插件市场 (索引 1.1)
```

`pack` 收录插件目录下除 `*.go`、`go.mod`、`go.sum`、构建脚本、密钥、隐藏文件与 `dist/` 以外的全部文件，zip 可复现（固定时间戳、排序）。签名算法见 [ABI.md §6](ABI.md#6-插件包与签名)。私钥永远不要提交或放进插件包。发布、版本、审核、密钥轮换与撤回见 [PUBLISHING.md](PUBLISHING.md)。

其他语言：任何能编译到 `wasm32-wasi`（或纯 `wasm32`）、导出 `memory` 与 `() -> i32` 处理函数、按 ABI 导入 `lounge.*` 的语言都可以（Rust、TinyGo、AssemblyScript、C）。
