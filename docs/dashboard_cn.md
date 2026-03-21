# Dashboard 与代理

[← 返回 README](../README_CN.md)

## Dashboard 登录

- 访问 `/dashboard/`
- 使用 `server.admin_password` 对应的管理密码登录
- 登录成功后，Dashboard 会通过 `dashboard_session` Cookie 访问 `/admin/*`

注意区分两套认证体系：

- `/v1/messages` 使用 API Key 认证，供外部程序调用
- `/admin/*` 使用 Dashboard 会话认证，仅供 Dashboard 前端使用

## Dashboard 能做什么

- 查看账号池总量、可用量、额度使用情况
- 查看账号详细信息、可用模型、研究额度状态
- 查看刷新进度与最近刷新时间
- 切换联网搜索、工作区搜索、调试日志
- 打开最佳账号或指定账号的代理会话

## 打开 Notion Web 代理

代理访问链路如下：

1. 在 Dashboard 中点击"最佳账号"或某个账号
2. 浏览器访问 `/proxy/start?best=true` 或 `/proxy/start?email=<邮箱>`
3. 服务端创建 `np_session`
4. 自动跳转到 `/ai`
5. 后续 HTML、API、资源和 WebSocket 都通过当前账号代理

反向代理会自动处理：

- 注入 `full_cookie` 或最小必需 Cookie
- 转发 `/_assets/*`、`/api/*`、`/primus-v8/*`、`/_msgproxy/*`
- 改写 Notion 前端里的基础地址
- 过滤 GTM、customer.io 等分析脚本
