# 交互所有权

| 能力 | 唯一实现 | 行为 |
|---|---|---|
| Form | web/index.html + app.js | 原生 required/pattern；提交忙碌禁用；保留输入 |
| Feedback | app.js notice / #notice | 文本 aria-live，错误与警告不会伪装成功 |
| CRUD | app.js api / renderEvidence | 只有新增和查询，存证不可修改删除 |
| Scrollbar | style.css 全局 | 标准属性与 WebKit 回退 |
| Search | #query 表单 | 完整 ID 显式提交，无远程联想；清除后回焦点 |
| Secret | #token | 默认遮罩，可显示，只保存在内存 |

查询和操作日志共享鉴权；操作成功后不清空原文，方便人工验证。HTTP 202 显示业务警告。核对功能不自动重发交易。
