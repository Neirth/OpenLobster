# OpenLobster — 个人 AI 助手

<p align="center">
    <picture>
        <source media="(prefers-color-scheme: light)" srcset="https://placehold.co/1600x200/ffffff/000000?text=OpenLobster&font=raleway">
         <img src="https://placehold.co/800x200/0b6e4f/ffffff?text=OpenLobster&font=raleway" alt="OpenLobster" width="800">
    </picture>
</p>

<p align="center">
  <strong>个人，自托管全能的 AI 助手 — 在任何您想运行的地方运行，连接到您使用的任何渠道。</strong>
</p>


<p align="center">
  <a href="README.en.md">English</a> •
  <a href="README.es.md">Español</a> •
  <a href="README.zh.md">简体中文</a>
</p>

<p align="center">
  <a href="https://github.com/Neirth/OpenLobster/actions/workflows/release.docker-images.yaml?branch=main"><img src="https://img.shields.io/github/actions/workflow/status/Neirth/OpenLobster/release.docker-images.yaml?branch=master&style=for-the-badge" alt="CI status"></a>
  <a href="https://github.com/Neirth/OpenLobster/releases"><img src="https://img.shields.io/github/v/release/Neirth/OpenLobster?include_prereleases&style=for-the-badge" alt="GitHub release"></a>
  <a href="https://neirth.gitbook.io/openlobster"><img src="https://img.shields.io/badge/Docs-GitBook-blue?style=for-the-badge" alt="Docs"></a>
  <a href="LICENSE.md"><img src="https://img.shields.io/badge/License-GPLv3-blue.svg?style=for-the-badge" alt="GPLv3 License"></a>
  <a href="https://discord.gg/Qx9eJcZH5v"><img src="https://img.shields.io/badge/Discord-%235865F2.svg?style=for-the-badge&logo=discord&logoColor=white" alt="Discord"></a>
  <a href="https://github.com/Neirth/OpenLobster/discussions"><img src="https://img.shields.io/badge/问答-1E232A?style=for-the-badge&logo=github&logoColor=white" alt="GitHub Discussions"></a>
</p>

> [!NOTE]
> **从 OpenClaw 迁移？** [Discussions #44](https://github.com/Neirth/OpenLobster/discussions/44) 提供了详细的迁移指南。

这是一个有态度的 OpenClaw 分支，实际解决了自该项目爆火以来人们一直抱怨的问题。

OpenClaw 曾风靡一时——自托管的AI智能体，炒作热烈，增长迅速。但是后来安全社区审查后，情况急转直下：大量的CVE漏洞填满了整个RedPacket页面，而且在其技能市场（ClawHub）中，26%的技能至少存在一个漏洞。其记忆系统只是一个在并发会话时会崩溃的 MEMORY.md 文件。“调度器”不过是个每30分钟唤醒一次读取 HEARTBEAT.md 清单的心跳守护进程。多用户支持基本上不存在——文档中竟然写着“只有主会话能写入 MEMORY.md，防止了并行会话冲突”，仿佛那是一个特性一样。

本分支最初是为了提供针对上述所有问题的个人修复版本而开始的，后来不断发展壮大。

---

## 有什么改变（以及为什么）

* **记忆** — MEMORY.md 和一个 markdown 文件夹不是记忆系统，那是维基百科。OpenLobster 使用适当的图形数据库（Neo4j），使得智能体能在与人交谈时构建节点、边缘和类型关系。你可以从 UI 浏览和编辑它。另外，也有本地使用的文件后端选项，无需运行 Neo4j。

* **多用户** — 在 OpenClaw 中，策展记忆仅加载到“隐私的主会话”中，且从不用于群组上下文。也没有记录分离聊天的独立用户的概念。在这里，您平台和频道的每个用户都被视为成熟的第一类实体，具有自己的对话记录，工具权限和配对流程。一个 Telegram 用户和一个 Discord 用户可以和同一个智能体聊天而互不干扰。

* **调度器 (Scheduler)** — 读取 HEARTBEAT.md 的每半小时的心跳循环已经消失。现在有了真正的多任务计划表支持，并允许通过 cron 表达式重复运行任务，或是支持 ISO 8601 时间的单次任务。状态、下次运行时间和日志在面板中均可见。

* **MCP** — OpenClaw的 MCP 集成本质上只是个演示。OpenLobster 连接到任意 HTTP MCP 流式服务器，支持完整的 OAuth 2.1 流程，允许浏览每个服务器的工具并为每个用户配置单独权限，从而精确控制谁能做啥。还内置了一键配置的市场集成。

* **安全** — 安全是一大改进。OpenClaw 默认关闭认证，这也是为何 Censys 上会有四万个暴露的实例。这里，仪表板默认启用不记名令牌认证 (`OPENLOBSTER_GRAPHQL_AUTH_TOKEN`)。配置与密钥在硬盘加密存储。API 密钥和频道词元存在后端的秘密服务（加密文件或 OpenBao），绝非明文的 YAML。环境变量 `OPENLOBSTER_*` 不会暴漏给任何终端工具。之前让无认证用户直击代理 API 的 CVE？在这儿不存在。

* **后端** — 原本的 OpenClaw 基于 Node.js/TypeScript。后端所有代码已用 Go 完全重写。这意味着单一静态二进制文件、无运行时依赖、启动更快、占用更小的运行内存，并使用 gqlgen 实现了恰当的 GraphQL API。这也使部署异常简明——告别 npm，告别 Node 版本的绑定，也免去了被 \`node_modules\` 烦扰。

* **UI 界面** — Web 界面也是从零开始搭建以保证实在的易用性。第一次使用引导程序自动加载配置面板以帮助避免碰到硬编码文件。设置呈现为动态表单——你只用关心你激活设置带来的字段要求就行。需要修改 YAML 达成的都直接可通过浏览器办到。

> [!NOTE]
> **需要贡献者** 我正在考虑为该存储库添加维护者。目前我们在 [Discussions #68](https://github.com/Neirth/OpenLobster/discussions/68) 进行讨论。

## 技术栈

| 层级 | 技术 |
| ----- | ---- |
| 前端 | SolidJS + Vite, 原生 CSS |
| 后端 | Go, GraphQL (gqlgen) |
| 数据库 | SQLite / PostgreSQL / MySQL |
| 记忆系统 | File (GML) 或 Neo4j |
| 机密数据 | 加密文件 或 OpenBao |
| 渠道 | Telegram, Discord, WhatsApp, Slack, Twilio SMS |
| 人工智能引擎 | OpenAI, Anthropic, Ollama, OpenRouter, Docker Model Runner, OpenAI-Compatible等 |


## 快速开始

```bash
# 安装依赖
pnpm install

# 构建前端 + 后端 (前端内置在可执行文件中)
pnpm build --filter=@openlobster/backend

# 仅构建前端
pnpm build --filter=@openlobster/frontend

# 全部构建
pnpm build

# 运行
./dist/openlobster
```

Web 的控制台地址一般在 `http://127.0.0.1:8080`。首次引导将帮助配置所有的环境。

## Docker

```bash
docker run -p 8080:8080 \
  -e OPENLOBSTER_GRAPHQL_HOST=0.0.0.0 \
  -e OPENLOBSTER_GRAPHQL_AUTH_TOKEN=您的机密令牌 \
  -v ~/.openlobster/data:/app/data \
  -v ~/.openlobster/workspace:/app/workspace \
  -d ghcr.io/neirth/openlobster/openlobster:latest
```

查看 `.docker/` 来获取对应的 Dockerfiles （如 `Dockerfile.basic` 作为最精简配置, `Dockerfile.static` 作为全静态执行）。

## 详细配置

配置设置可以直接在仪表盘进行。但也允许注入 `OPENLOBSTER_` 的环境变量。Viper 引擎能够协助自发匹配转换（如 YAML 中的连接点，变为这里下划线表达）。

```bash
# 精简示例
OPENLOBSTER_AGENT_NAME=my-agent
OPENLOBSTER_DATABASE_DRIVER=sqlite
OPENLOBSTER_DATABASE_DSN=./data/openlobster.db
OPENLOBSTER_GRAPHQL_AUTH_TOKEN=您的机密令牌

# 人工智能供应商平台（选一）
OPENLOBSTER_PROVIDERS_OLLAMA_ENDPOINT=http://localhost:11434
OPENLOBSTER_PROVIDERS_OLLAMA_DEFAULT_MODEL=llama3.2:latest
```

<details>
<summary>详细环境变量参照</summary>

| 变量 | YAML 参数 | 描述 |
| -------- | -------- | ----------- |
| `OPENLOBSTER_AGENT_NAME` | `agent.name` | 代理面板命名 |
| `OPENLOBSTER_DATABASE_DRIVER` | `database.driver` | `sqlite`, `postgres`, `mysql` |
| `OPENLOBSTER_DATABASE_DSN` | `database.dsn` | 连结内容字符串 |
| `OPENLOBSTER_DATABASE_MAX_OPEN_CONNS` | `database.max_open_conns` | 极限链接量 |
| `OPENLOBSTER_DATABASE_MAX_IDLE_CONNS` | `database.max_idle_conns` | 极限闲置连接 |
| `OPENLOBSTER_MEMORY_BACKEND` | `memory.backend` | `file` 或者 `neo4j` |
| `OPENLOBSTER_MEMORY_FILE_PATH` | `memory.file.path` | 文件系统的存储路径 |
| `OPENLOBSTER_MEMORY_NEO4J_URI` | `memory.neo4j.uri` | 例如 `bolt://localhost:7687` |
| `OPENLOBSTER_MEMORY_NEO4J_USER` | `memory.neo4j.user` | Neo4j 的名称 |
| `OPENLOBSTER_MEMORY_NEO4J_PASSWORD` | `memory.neo4j.password` | Neo4j 密码 |
| `OPENLOBSTER_SECRETS_BACKEND` | `secrets.backend` | `file` 还是 `openbao` |
| `OPENLOBSTER_SECRETS_FILE_PATH` | `secrets.file.path` | 密码锁柜的路线 |
| `OPENLOBSTER_SECRETS_OPENBAO_URL` | `secrets.openbao.url` | OpenBao 目标伺服器 |
| `OPENLOBSTER_SECRETS_OPENBAO_TOKEN` | `secrets.openbao.token` | OpenBao 的认证代币 |
| `OPENLOBSTER_PROVIDERS_OPENAI_API_KEY` | `providers.openai.api_key` | OpenAI Key |
| `OPENLOBSTER_PROVIDERS_OPENAI_MODEL` | `providers.openai.model` | 例如 `gpt-4o` |
| `OPENLOBSTER_PROVIDERS_OPENAI_BASE_URL` | `providers.openai.base_url` | 可定底栈连接 |
| `OPENLOBSTER_PROVIDERS_OPENROUTER_API_KEY` | `providers.openrouter.api_key` | OpenRouter Key |
| `OPENLOBSTER_PROVIDERS_OPENROUTER_DEFAULT_MODEL` | `providers.openrouter.default_model` | 例如 `openai/gpt-4o` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_ENDPOINT` | `providers.ollama.endpoint` | 例如 `http://localhost:11434` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_DEFAULT_MODEL` | `providers.ollama.default_model` | 例如 `llama3.2:latest` |
| `OPENLOBSTER_PROVIDERS_OLLAMA_API_KEY` | `providers.ollama.api_key` | 附载验证要求 |
| `OPENLOBSTER_PROVIDERS_ANTHROPIC_API_KEY` | `providers.anthropic.api_key` | Anthropic key |
| `OPENLOBSTER_PROVIDERS_ANTHROPIC_MODEL` | `providers.anthropic.model` | 例如 `claude-sonnet-4-6` |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_API_KEY` | `providers.openaicompat.api_key` | OpenAI兼容服务端 key |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_BASE_URL` | `providers.openaicompat.base_url` | Base URL |
| `OPENLOBSTER_PROVIDERS_OPENAICOMPAT_MODEL` | `providers.openaicompat.model` | 模型名字 |
| `OPENLOBSTER_PROVIDERS_DOCKER_MODEL_RUNNER_ENDPOINT` | `providers.docker_model_runner.endpoint` | DMR 端点 |
| `OPENLOBSTER_PROVIDERS_DOCKER_MODEL_RUNNER_DEFAULT_MODEL` | `providers.docker_model_runner.default_model` | DMR 模型 |
| `OPENLOBSTER_GRAPHQL_ENABLED` | `graphql.enabled` | 是否开放 GraphQL API |
| `OPENLOBSTER_GRAPHQL_PORT` | `graphql.port` | 默认 `8080` |
| `OPENLOBSTER_GRAPHQL_HOST` | `graphql.host` | 默认用 `127.0.0.1` |
| `OPENLOBSTER_GRAPHQL_BASE_URL` | `graphql.base_url` | 外部供 OAuth 回录点 |
| `OPENLOBSTER_GRAPHQL_AUTH_ENABLED` | `graphql.auth_enabled` | 需要提供前端校验令牌与否 |
| `OPENLOBSTER_GRAPHQL_AUTH_TOKEN` | `graphql.auth_token` | 不记名连接用识别代币 |
| `OPENLOBSTER_LOGGING_LEVEL` | `logging.level` | `debug`, `info`, `warn` 亦或是 `error` |
| `OPENLOBSTER_LOGGING_PATH` | `logging.path` | 日志记载存放路标 |
| `OPENLOBSTER_WORKSPACE_PATH` | `workspace.path` | 操作使用端路标 |
| `OPENLOBSTER_CONFIG_ENCRYPT` | — | `1` （默认值）磁盘化参数加密，亦或是输入 `0` 回流原始文格式加载 |

</details>

## 渠道
本助手可随意游走各客户端系统。去往设定系统自由控制调整。

- **Telegram** — 同 `@BotFather` 搭建聊天人并输入获取代证即可连入使用，并包含组群通信系统。
- **Discord** — 去向 开发者中心，创造聊天客户端即可纳入其中加入房间游玩使用。
- **Slack** — 基于 Socket 并利用机器人(`xoxb-`) 和应用程序内代币来完成。
- **WhatsApp** — 建立由 WhatsApp Business API 开发的连接进入该社交圈使用。
- **Twilio SMS** — 基础的标准短信功能以用于电话号码联络发送。

## 用户相关详细材料
可以进到目录下的 `docs/` 去翻阅关于网页面板搭建及相关操控，这囊括各种指令的使用、插件设置管理或是计划预设分配等等的控制方式，由于为 Gitbook 构建，但您大可直接使用普通的 markdown 工具轻松阅读它。

## 常问疑问解答 (FAQ)

**我可否直接在 OpenLobster 上部署我 OpenClaw 中的既有设定?**

并不允许，因为在架构处理上由于差距跨度无法相互转移对应的设定模块。包括用户的使用机制授权权限等问题都已经重新整理编写因此你需要借由手工转档的方式逐步录入系统当中。

**我可以剥除 Neo4j 后继续在系统上使用吗?**

可以的。更改为 `OPENLOBSTER_MEMORY_BACKEND=file` 及向系统提供设定位置的 `OPENLOBSTER_MEMORY_FILE_PATH`，该档资料系统即转以建立 GML 为数据包并本地处理保留。作为小基建已是足敷；而若是想应付有巨大图网络或多人连接交互请求才会呼吁利用 Neo4j 以确保效率。

### 这些可在我的微型装置内游玩实现?

是的！作为完全基于 Go 单一程序执行的文件而言他异常快速！

**详细参数基准：**
- 开启费时: 只需 200毫秒 (以前 Node.js 处理的 OpenClaw 会消磨掉数秒钟)
- RAM： 全部设定完毕满载只要不到 30MB 左右 (对比过去往往耗费惊人的 >150MB+ 以上)
- 独立单档容量规模: ~66MB 占用量 (Node.js由于挂载 node_modules 以致突破 200MB+)

可称其简直是为了下列硬件度身定制：
- 树莓派 3代 / 4代 (Raspberry Pi 3/4)
- 不足 512MB RAM 的边缘 VPS 主机
- 空间极尽限制下的各种 NAS
- 一件标价不到15块美元的 LicheeRV Nano 微机也依然可以使用

**我可以套用何种人工智能模块？**

目前能连接 OpenAI、Anthropic、Ollama、OpenRouter、Docker Model Runner 及各式各种和 OpenAI 进行对接串流接口等，随用随换，仅受制于单一时刻只可承接一个来源的引擎运作。

**这是否代表 GraphQL 已经向全球开放请求？**

从默认状况下是的！如果您想防护阻档就向参数宣告增加保护并设定如 `OPENLOBSTER_GRAPHQL_AUTH_TOKEN` 并挂载安全且强而有力的秘钥！一经设立则您每次要求存取操作皆得经借校验流程验证方得放行使用，而这也是我们对挂处于互联网上的各位提出的第一个建议安全准则措施要求之一，并在此注明其纯属为 UI 的控制手段，请避免以对待外部开放端点作操作。

**假如本间企业期望入驻相关外挂功能入市场该做作什么?**

若作为赞助支持本开源系统的开发单位我们将会欢迎您的 PR（即去对 `apps/frontend/public/marketplace.json` 进行申请处理），否则的话我们会婉拒由于后期的代码检查保养维运需巨大精力支持而恕不处理其他未经审核要求之提交（首日上市阶段的模块属早期善意投入并无关本条例限制）。

**我有私心属意特定外援外置插件的请求加入意愿要如果办？**

非常欢迎随时提立需求以向我们建档发声这该工具的利益以及用途目的以换取社群赞同认可的话我们一样将会收录而毋须金元捐献！

**能请问系统一直陈述的 “结对(pairing)” 究竟为何意?**

任何使用渠道平台端的人们初来乍到呼叫连线使用系统助手之后所开启的操作称为结对，这就等于让例如群组里的某个特定成员获取自己的专署身分凭据来开启接下来的系统对话存取流程及控制，这大大有别系统早年只有个主从一齐覆盖导致冲突的糟糕状态而已有专有的环境独立出来。

**有版本升级要怎搞？**

简单重新拖移系统中的文件或者是借由拉取并更替 Docker 最新文档后重新发动连线（即重启命令指令）执行它。数据库等部分在系统发现状况后其自身皆具备自动修补升级功能机制并不劳烦手动！

## 许可证
详情见 [LICENSE.md](LICENSE.md)。
