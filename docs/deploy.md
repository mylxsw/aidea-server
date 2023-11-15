# 部署指南

> 建议大家尽可能的自己去部署，遇到问题在 [GitHub Issues](https://github.com/mylxsw/aidea-server/issues) 提出，如果实在懒得搞，可以找我来帮你部署，详情参考 [服务器代部署说明](./deploy-vip.md)。
> 
> 由于时间仓促，文档可能尚未详尽，我将在后续逐步补充详细的说明文档。

## 架构草图

![image](https://github.com/mylxsw/aidea-server/assets/2330911/ffb59bb3-46d7-4fe6-a777-b409acff17e2)

## 项目依赖

![image](https://github.com/mylxsw/aidea-server/assets/2330911/43c095f5-4964-46c7-8c50-9b44b6d36fef)

必选依赖

- MySQL 5.7+
- Redis 7.0+ （低版本兼容性未知）
- OpenAI Key
- 七牛云对象存储 Kodo 服务 （需要配置以下图片样式）

  名称 |接口
  :---:|:---:
  avatar | imageView2/1/w/400/h/400/format/webp/q/75
  thumb | imageView2/2/w/1280/h/1280/format/webp/interlace/1/q/80\|imageslim
  thumb_500 | imageView2/2/w/500/h/500/format/webp/q/75
  square_500 | imageView2/1/w/500/h/500/format/jpg/q/75
  fix_square_1024 | imageMogr2/auto-orient/thumbnail/!1024x1024r/gravity/Center/crop/1024x1024/blur/1x0/quality/75

可选依赖

- 邮件服务器（邮箱登录、注册功能暂未开放）
- 短信服务（如需注册功能，则以下至少有一个）
    - 阿里云短信服务
    - 腾讯云短信服务
- 内容安全检测（使用阿里云的内容安全服务，用于检测提示语中是否包含敏感词汇）
- 有道翻译 API 接口（翻译功能、文生图及图生图提示语中文转英文）
- 百度文心千帆大模型 Keys，支持以下模型 【[开通指南](https://github.com/mylxsw/aidea-server/wiki/百度文心千帆服务开通指南)】
    - model_ernie_bot_turbo
    - model_ernie_bot
- 阿里灵积平台模型 Keys，支持以下模型
    - qwen-v1
- 讯飞星火大语言模型 Keys，支持以下模型
    - general
    - generalv2
- Anthropic API Keys，支持以下模型
    - claude-instant
    - cluade-2.0
- [DeepAI](https://deepai.org/) 平台 Keys，用于图片超分辨率、上色
- [Stability AI](https://stability.ai/) Stable Diffusion 官方提供的 API，用于 SDXL 1.0  模型文生图、图生图
- [Leap](https://tryleap.ai/) 平台 Keys，用于 Leap 平台提供的文生图、图生图模型
- [Fromston](https://fromston.6pen.art/) 国内 6pen 团队提供的 Keys，用于文生图、图生图模型
- [getimg.ai](https://getimg.ai/tools/api) 平台 Keys，用于文生图、图生图模型
- [支付宝在线支付](./alipay-configuration.md)

## 部署步骤

### 1. 初始化 MySQL 数据库

按顺序执行 **migrate/2023090801-ddl.sql** 和 **migrate/2023090802-dml.sql** 两个 SQL 文件，完成数据库的初始化。

这里以 MySQL 命令行的方式为例：

```bash
mysql> CREATE DATABASE aidea_server CHARSET utf8mb4;
mysql> USE aidea_server;
mysql> SOURCE /Users/mylxsw/Workspace/codes/ai/ai-server/migrate/2023090801-ddl.sql;
mysql> SOURCE /Users/mylxsw/Workspace/codes/ai/ai-server/migrate/2023090802-dml.sql;
mysql> SOURCE /Users/mylxsw/Workspace/codes/ai/ai-server/migrate/2023092501-dml.sql;
```

### 2. 创建配置文件

以 **config.yaml** 为范例，修改配置文件，放置在服务器的任意目录（建议目录 `/etc/aidea-server.yaml`）。

> 完整配置选项参考 [cmd/main.go](https://github.com/mylxsw/aidea-server/blob/master/cmd/main.go) 文件。

### 3. 启动服务

将编译好的软件包放置在服务器的任意目录（建议目录 `/usr/local/bin/aidea-server`），执行以下命令启动服务

```bash
/usr/local/bin/aidea-server --conf /etc/aidea-server.yaml
```

> 也可以使用 Docker 容器启动服务，该部分文档待补充。

## 常见问题

1. 部署过程中遇到问题，不知道该如何解决

    请在 [GitHub Issues](https://github.com/mylxsw/aidea-server/issues) 提出你的问题，有时间的时候我会尽快回复。
2. 部署文档不详细，什么时候补充？

    有空的时候会补充，但是不保证时间，大家普遍遇到的问题会随时更新。
3. 是否支持 Docker 一键部署？
  
    暂时没有提供，但是会有的，有时间了会更新，也欢迎大家贡献。
4. 部署了服务端之后，客户端要怎么修改才能使用自己的服务端呢？
    
    请 Fork 项目 [mylxsw/aidea](https://github.com/mylxsw/aidea)，然后修改 `lib/helper/constant.dart` 文件，找到 `apiServerURL` 常量，修改为自己服务器的地址，然后重新打包客户端即可。
    
    ```dart
    // API 服务器地址
    const apiServerURL = 'https://api.aidea.com';
    ```
5. 我不想自己安装，能否帮我部署一套？
    
    建议大家尽可能的自己去部署，遇到问题在 [GitHub Issues](https://github.com/mylxsw/aidea-server/issues) 提出，如果实在懒得搞，可以找我来帮你部署，详情参考 [服务器代部署说明](./deploy-vip.md)。
