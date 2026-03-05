# AIdea Server - AI Chat, Collaboration & Image Generation

English | [简体中文](./README.zh-CN.md)

<a href="https://trendshift.io/repositories/855" target="_blank"><img src="https://trendshift.io/api/badge/repositories/855" alt="mylxsw%2Faidea-server | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>

A fully open-source APP server built with Golang that integrates mainstream large language models and image generation models.

Try it out:

https://aidea.aicode.cc

Open-source repositories:

- Client: https://github.com/mylxsw/aidea
- Server: https://github.com/mylxsw/aidea-server
- Docker deployment: https://github.com/mylxsw/aidea-docker

## Self-hosting

If you prefer not to use the managed cloud service, you can deploy the server yourself. [See the deployment guide here](./docs/deploy.md).

If you'd rather have someone else handle the setup, feel free to reach out for assisted deployment. See [Assisted Server Deployment](./docs/deploy-vip.md) for details.

### Community

- WeChat tech discussion group:

    <img src="https://github.com/user-attachments/assets/379d0b66-b806-4ed4-ae2e-30fccd9de50e" width="400"/>

    If you cannot join, add WeChat ID `x-prometheus` as a friend and you'll be invited to the group.

- WeChat Official Account

    <img src="https://github.com/mylxsw/aidea-server/assets/2330911/376a3b9f-eacd-45c6-9630-39eb720ba097" width="500" />

## About the Code

> Code comments and technical documentation are currently limited and will be supplemented over time. Please note the following points to avoid confusion:
>
> - `Room` and `Advisory Group` in the code both refer to `Digital Persona`. Due to multiple revisions, the naming evolved: `Room` → `Advisory Group` → `Digital Persona`.
> - The v1 version of `Creation Island` is entirely different from v2. v1 served App versions up to 1.0.1; starting from 1.0.2 it is no longer used, hence the v2 version.

Frameworks used in this project:

- [Glacier Framework](https://github.com/mylxsw/glacier): An in-house modular application development framework with dependency injection support, built on the [go-ioc](https://github.com/mylxsw/go-ioc) IoC container. It solves dependency propagation and modularization for Go applications.
- [Eloquent ORM](https://github.com/mylxsw/eloquent): An in-house code-generation-based ORM framework inspired by the Laravel PHP framework, with support for MySQL and other databases.

Code structure:

| Directory        | Description                                                                                             |
|------------------|---------------------------------------------------------------------------------------------------------|
| api              | OpenAI-compatible API; endpoints here can be used directly by any third-party software that supports the OpenAI API protocol |
| server           | API endpoints provided for the AIdea client application                                                 |
| config           | Configuration definitions and management                                                                |
| migrate          | Database migration files (SQL)                                                                          |
| cmd              | Application entry point                                                                                 |
| pkg              | Public packages that can be imported by other projects                                                  |
| ⌞ ai             | AI model interface implementations for various providers                                                |
| ⌞ ai/chat        | Abstract chat model interface; all chat models are wrapped here to be compatible with the OpenAI Chat Stream protocol |
| ⌞ aliyun         | Alibaba Cloud SMS and content-safety service implementations                                            |
| ⌞ dingding       | DingTalk notification bot                                                                               |
| ⌞ misc           | Miscellaneous helper functions                                                                          |
| ⌞ jobs           | Scheduled tasks, e.g. daily user token-consumption statistics                                           |
| ⌞ mail           | Email sending                                                                                           |
| ⌞ proxy          | SOCKS5 proxy implementation                                                                             |
| ⌞ rate           | Rate-limiting implementation                                                                            |
| ⌞ redis          | Redis instance                                                                                          |
| ⌞ repo           | Data model layer; encapsulates all database operations                                                  |
| ⌞ repo/model     | Data model definitions using [mylxsw/eloquent](https://github.com/mylxsw/eloquent)                     |
| ⌞ service        | Service layer for logic that doesn't belong in the Controller or Repo layers                            |
| ⌞ sms            | Unified SMS service abstraction that hides the underlying SMS provider implementation from business logic |
| ⌞ tencent        | Tencent speech-to-text and SMS service implementations                                                  |
| ⌞ token          | JWT Token                                                                                               |
| ⌞ uploader       | File upload/download backed by Qiniu Cloud Storage                                                      |
| ⌞ voice          | Text-to-speech backed by Qiniu Cloud (currently disabled)                                               |
| ⌞ youdao         | Youdao Translation API implementation                                                                   |
| internal         | Internal packages; only available within this project                                                   |
| ⌞ queue          | Task queue implementation; all asynchronously processed tasks are defined here                          |
| ⌞ queue/consumer | Task queue consumers                                                                                     |
| ⌞ payment        | Online payment service implementations (e.g. Alipay, Apple)                                            |
| ⌞ coins          | Service pricing and billing policies                                                                    |
| config.yaml      | Example configuration file                                                                              |
| coins-table.yaml | Example pricing table configuration                                                                     |
| nginx.conf       | Example Nginx configuration                                                                             |
| systemd.service  | Example Systemd service configuration                                                                   |

Build the project:

```bash
go build -o build/debug/aidea-server cmd/main.go
```

## App Screenshots

Light theme

![image](https://github.com/mylxsw/aidea-server/assets/2330911/9c9e878c-67ab-43d6-a9d0-84faf9a6a511)

Dark theme

![image](https://github.com/mylxsw/aidea-server/assets/2330911/9e5cc989-4ef5-496b-ab4d-7b9d29793ce3)


## Star History

<a href="https://star-history.com/#mylxsw/aidea-server">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=mylxsw/aidea-server&type=Date&theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=mylxsw/aidea-server&type=Date" />
    <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=mylxsw/aidea-server&type=Date" />
  </picture>
</a>

## License

MIT

Copyright (c) 2023, mylxsw
