# Steam Go SDK

协议源位于 `../steam-proto`。修改协议后运行 `make generate`，不要手改生成文件。

## HTTP 路径兼容

Kratos v3.0.0 的表单编码使用 protobuf JSON 字段名（如 `appId`），而上游 HTTP 生成器使用 proto 路径变量（如 `{app_id}`）。直接调用 `http.BuildPath` 会生成缺失 AppID 的路径，并错误地把 `appId` 留在 query 中。

`cmd/protoc-gen-go-http-compat` 委托固定版本的上游插件生成代码，然后通过 Go AST 将生成客户端的 `BuildPath` 调用接入 `internal/httpbinding`。适配器按 protobuf 描述符解析路径字段，使用与 Kratos 编码器一致的名字；支持嵌套字段和显式 `json_name`。服务端路由、PathTemplate 元数据及 JSON 契约保持原样。所有生成文件仍由 `make generate` 统一产出。

2026-09-20 复核上游：Go 模块代理的 `latest` 与 `main` 都是 `v3.0.0-20260626125723-668db92c2c00`。该版本的 `BuildPath` 仍直接使用模板字段名读取 JSON-name query map，问题未修复。官方讨论 [#3750](https://github.com/orgs/go-kratos/discussions/3750) 描述了同一根因，但没有形成维护者认可的方案；相关 [PR #3311](https://github.com/go-kratos/kratos/pull/3311) 面向旧版 `EncodeURL`、存在冲突且尚未合并。因此保留兼容层，同时把委托生成器升级到这个最新上游提交。

工作范围：SDK 生成配置、兼容生成器、路径适配器、生成结果及回归测试。依赖既有 `steam-proto` 契约和 Kratos v3.0.0，不修改协议。

## 验证

```sh
make generate
make test
go vet ./...
make integration-test
```

SDK 测试直接调用生成客户端，检查在线人数、评论、SteamDB、Todo 请求的真实 URL、query 编码及请求体分离。`integration-test` 需要同级 `steam-api`，自动创建临时 Go workspace，将本地 SDK 连到真实 API service/usecase，验证 0、160 和 NOT_FOUND 及实际来源；退出后删除 workspace，不修改模块依赖。

`steam-api` 当前仍引用已发布的 SDK v0.2.2。修复提交后还需发布新版 SDK 并升级使用 HTTP 客户端的项目，旧版本不会自动获得修复。API 中暂以 `sdk_workspace` build tag 保存本地新版 SDK 的集成回归，发布并升级依赖后可移除此隔离。
