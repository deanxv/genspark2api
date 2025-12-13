<p align="right">
   <strong>中文 | <a href="README_EN.md">English</a></strong> 
</p>
<div align="center">

# Genspark2API

_觉得有点意思的话 别忘了点个 ⭐_

<a href="https://t.me/+LGKwlC_xa-E5ZDk9">
  <img src="https://img.shields.io/badge/Telegram-AI Wave交流群-0088cc?style=for-the-badge&logo=telegram&logoColor=white" alt="Telegram 交流群" />
</a>

<sup><i>AI Wave 社群</i></sup> · <sup><i>(群内提供公益API、AI机器人)</i></sup>

---

**Genspark2API** 是一个使用 Go 语言编写的高性能代理服务，用于将 **Genspark.ai** 的能力封装为 **兼容 OpenAI 标准的 API 接口**。

### 🌟 核心特性

- 🤖 **对话大模型**：支持 GPT-5.1 系列、Claude Opus/Sonnet 4.5、Gemini 3、O3-Pro 等主流模型
- 🖼️ **文生图能力**：支持 DALL·E、Flux、Imagen4 等多种绘图模型
- 🎬 **文/图生视频**：支持 Sora-2、Veo3、Kling、Runway 等视频生成模型
- 🔄 **OpenAI API 兼容**：可直接接入任何兼容 OpenAI 的客户端/中间件
- 🛡️ **智能防护绕过**：内置 Cloudflare 与 ReCaptcha v3 相关处理逻辑（需按文档配置）
- 🔀 **多 Cookie 池轮询**：自动在多个账号之间分流，请求失败自动切换重试
- 🔧 **Tool Calling 支持**：支持 OpenAI 风格的工具调用（function calling）
- 🌐 **联网搜索**：模型名后加 `-search` 即可开启联网搜索（如: `gpt-5.1-search`）
- 📊 **流式响应**：支持 SSE 流式输出，体验与官方 OpenAI 接口一致

### 📦 快速开始

```bash
docker run -d --name genspark2api \
  -p 7055:7055 \
  -e GS_COOKIE="你的_genspark_cookie" \
  -e API_SECRET="你的_api_secret" \
  deanxv/genspark2api:latest
```

</div>

> ⚠️目前官方强制校验`ReCaptchaV3`
> 不通过则模型降智/生图异常,请参考[genspark-playwright-prxoy服务过V3验证](#genspark-playwright-prxoy服务过V3验证)并配置环境变量
`RECAPTCHA_PROXY_URL`。

## 功能

- [x] 支持对话接口(流式/非流式)(`/chat/completions`)(请求非以下列表的模型会触发`Mixture-of-Agents`模式)
    - **gpt-5.1-low**
    - **gpt-5.1**
    - **gpt-5.1-high**
    - **gpt-5-pro**
    - **gpt-5.2**
    - **gpt-5.2-pro**
    - ~~**gpt-5-minimal**~~
    - ~~**gpt-5**~~
    - ~~**gpt-5-high**~~
    - ~~**gpt-4.1**~~
    - ~~**o1**~~
    - ~~**o3**~~
    - **o3-pro**
    - ~~**o4-mini-high**~~
    - ~~**claude-3-7-sonnet-thinking**~~
    - ~~**claude-3-7-sonnet**~~
    - **claude-sonnet-4-5**
    - ~~**claude-sonnet-4-thinking**~~
    - ~~**claude-sonnet-4**~~
    - **claude-opus-4-5**
    - **claude-opus-4-1**
    - **claude-4-5-haiku**
    - ~~**claude-opus-4**~~
    - **gemini-2.5-pro**
    - **gemini-3-pro-preview**
    - ~~**gemini-2.5-flash**~~
    - ~~**gemini-2.0-flash**~~
    - ~~**deep-seek-v3**~~
    - ~~**deep-seek-r1**~~
    - **grok-4-0709**
- [x] 支持**联网搜索**,在模型名后添加`-search`即可(如:`gpt-4o-search`)
- [x] 支持识别**图片**/**文件**多轮对话
- [x] 支持文生图接口(`/images/generations`)
    - **fal-ai/nano-banana**
    - **fal-ai/bytedance/seedream/v4**
    - **gpt-image-1**
    - **flux-pro/ultra**
    - **flux-pro/kontext/pro**
    - **imagen4**
- [x] 支持文/图生视频接口(`/videos/generations`),详情查看[文/图生视频请求格式](#生视频请求格式)
- [x] 支持自定义请求头校验值(Authorization)
- [x] 支持cookie池(随机)
- [x] 支持请求失败自动切换cookie重试(需配置cookie池)
- [x] 可配置自动删除对话记录
- [x] 可配置代理请求(环境变量`PROXY_URL`)
- [x] 可配置Model绑定Chat(解决模型自动切换导致**降智**),详细请看[进阶配置](#解决模型自动切换导致降智问题)。

### 接口文档:

略

### 示例:

<span><img src="docs/img2.png" width="800"/></span>

## 如何使用

略

## 如何集成NextChat

填 接口地址(ip:端口/域名) 及 API-Key(`PROXY_SECRET`),其它的随便填随便选。

> 如果自己没有搭建NextChat面板,这里有个已经搭建好的可以使用 [NeatChat](https://ai.aytsao.cn/)

<span><img src="docs/img5.png" width="800"/></span>

## 如何集成one-api

填 `BaseURL`(ip:端口/域名) 及 密钥(`PROXY_SECRET`),其它的随便填随便选。

<span><img src="docs/img3.png" width="800"/></span>

## 部署

### 基于 Docker-Compose(All In One) 进行部署

```shell
docker-compose pull && docker-compose up -d
```

#### docker-compose.yml

```docker
version: '3.4'

services:
  genspark2api:
    image: deanxv/genspark2api:latest
    container_name: genspark2api
    restart: always
    ports:
      - "7055:7055"
    volumes:
      - ./data:/app/genspark2api/data
    environment:
      - GS_COOKIE=******  # cookie (多个请以,分隔)
      - API_SECRET=123456  # [可选]接口密钥-修改此行为请求头校验的值(多个请以,分隔)
      - TZ=Asia/Shanghai
```

### 基于 Docker 进行部署

```docker
docker run --name genspark2api -d --restart always \
-p 7055:7055 \
-v $(pwd)/data:/app/genspark2api/data \
-e GS_COOKIE=***** \
-e API_SECRET="123456" \
-e TZ=Asia/Shanghai \
deanxv/genspark2api
```

其中`API_SECRET`、`GS_COOKIE`修改为自己的。

如果上面的镜像无法拉取,可以尝试使用 GitHub 的 Docker 镜像,将上面的`deanxv/genspark2api`替换为
`ghcr.io/deanxv/genspark2api`即可。

### 部署到第三方平台

<details>
<summary><strong>部署到 Zeabur</strong></summary>
<div>

[![Deployed on Zeabur](https://zeabur.com/deployed-on-zeabur-dark.svg)](https://zeabur.com?referralCode=deanxv&utm_source=deanxv)

> Zeabur 的服务器在国外,自动解决了网络的问题,~~同时免费的额度也足够个人使用~~

1. 首先 **fork** 一份代码。
2. 进入 [Zeabur](https://zeabur.com?referralCode=deanxv),使用github登录,进入控制台。
3. 在 Service -> Add Service,选择 Git（第一次使用需要先授权）,选择你 fork 的仓库。
4. Deploy 会自动开始,先取消。
5. 添加环境变量

   `GS_COOKIE:******`  cookie (多个请以,分隔)

   `API_SECRET:123456` [可选]接口密钥-修改此行为请求头校验的值(多个请以,分隔)(与openai-API-KEY用法一致)

保存。

6. 选择 Redeploy。

</div>


</details>

<details>
<summary><strong>部署到 Render</strong></summary>
<div>

> Render 提供免费额度,绑卡后可以进一步提升额度

Render 可以直接部署 docker 镜像,不需要 fork 仓库：[Render](https://dashboard.render.com)

</div>
</details>

## 配置

### 环境变量

1. `PORT=7055`  [可选]端口,默认为7055
2. `DEBUG=true`  [可选]DEBUG模式,可打印更多信息[true:打开、false:关闭]
3. `API_SECRET=123456`  [可选]接口密钥-修改此行为请求头(Authorization)校验的值(同API-KEY)(多个请以,分隔)
4. `GS_COOKIE=******`  cookie (多个请以,分隔)
5. `AUTO_DEL_CHAT=0`  [可选]对话完成自动删除(默认:0)[0:关闭,1:开启]
6. `REQUEST_RATE_LIMIT=60`  [可选]每分钟下的单ip请求速率限制,默认:60次/min
7. `PROXY_URL=http://127.0.0.1:10801`  [可选]代理
8. `RECAPTCHA_PROXY_URL=http://127.0.0.1:7022`  [可选]genspark-playwright-prxoy验证服务地址，仅填写域名或ip:端口即可。(
   示例:`RECAPTCHA_PROXY_URL=https://genspark-playwright-prxoy.com`或`RECAPTCHA_PROXY_URL=http://127.0.0.1:7022`)
   ,详情请看[genspark-playwright-prxoy服务过V3验证](#genspark-playwright-prxoy服务过V3验证)
9. `AUTO_MODEL_CHAT_MAP_TYPE=1`  [可选]自动配置Model绑定Chat(默认:1)[0:关闭,1:开启]
10. `MODEL_CHAT_MAP=claude-3-7-sonnet=a649******00fa,gpt-4o=su74******47hd`  [可选]Model绑定Chat(多个请以,分隔)
    ,详细请看[进阶配置](#解决模型自动切换导致降智问题)
11. `ROUTE_PREFIX=hf`  [可选]路由前缀,默认为空,添加该变量后的接口示例:`/hf/v1/chat/completions`
12. `RATE_LIMIT_COOKIE_LOCK_DURATION=600`  [可选]到达速率限制的cookie禁用时间,默认为600s
13. `REASONING_HIDE=0`  [可选]**隐藏**推理过程(默认:0)[0:关闭,1:开启]

~~14.
`SESSION_IMAGE_CHAT_MAP=aed9196b-********-4ed6e32f7e4d=0c6785e6-********-7ff6e5a2a29c,aefwer6b-********-casds22=fda234-********-sfaw123`  [可选]
Session绑定Image-Chat(多个请以,分隔),详细请看[进阶配置](#生图模型配置)~~

~~15. `YES_CAPTCHA_CLIENT_KEY=******`  [可选]YesCaptcha Client Key
过谷歌验证,详细请看[使用YesCaptcha过谷歌验证](#使用YesCaptcha过谷歌验证)~~

### cookie获取方式

1. 打开**F12**开发者工具。
2. 发起对话。
3. 点击ask请求,请求头中的**cookie**即为环境变量**GS_COOKIE**所需值。

> **【注】** 其中`session_id=f9c60******cb6d`是必须的,其他内容可要可不要,即环境变量`GS_COOKIE=session_id=f9c60******cb6d`


![img.png](docs/img.png)

## 进阶配置

### 解决模型自动切换导致降智问题

#### 方案一 (默认启用此配置)【推荐】

> 配置环境变量 **AUTO_MODEL_CHAT_MAP_TYPE=1**
>
> 此配置下,会在调用模型时获取对话的id,并绑定模型。

#### 方案二

> 配置环境变量 MODEL_CHAT_MAP
>
> 【作用】指定对话,解决模型自动切换导致降智问题。

1. 打开**F12**开发者工具。
2. 选择需要绑定的对话的模型(示例:`claude-3-7-sonnet`),发起对话。
3. 点击ask请求,此时最上方url中的`id`(或响应中的`id`)即为此对话唯一id。
   ![img.png](docs/img4.png)
4. 配置环境变量 `MODEL_CHAT_MAP=claude-3-7-sonnet=3cdcc******474c5` (多个请以,分隔)

### genspark-playwright-prxoy服务过V3验证

1. docker部署genspark-playwright-prxoy

#### docker

```docker 
docker run --name genspark-playwright-proxy -d --restart always \
-p 7022:7022 \
-v $(pwd)/data:/app/genspark-playwright-proxy/data \
-e PROXY_URL=http://account:pwd@ip:port #  [可选] 推荐(住宅)动态代理,配置代理后过验证概率更高,但响应会变慢。
-e TZ=Asia/Shanghai \
deanxv/genspark-playwright-proxy
```

#### docker-compose

```docker-compose
version: '3.4'

services:
  genspark-playwright-prxoy:
    image: deanxv/genspark-playwright-proxy:latest
    container_name: genspark-playwright-prxoy
    restart: always
    ports:
      - "7022:7022"
    volumes:
      - ./data:/app/genspark-playwright-prxoy/data
    environment:
      - PROXY_URL=http://account:pwd@ip:port #  [可选] 推荐(住宅)动态代理,配置代理后过验证概率更高,但响应会变慢。
```

2. 部署后配置`genspark2api`环境变量`RECAPTCHA_PROXY_URL`，仅填写域名或ip:端口即可。(示例:
   `RECAPTCHA_PROXY_URL=https://genspark-playwright-prxoy.com`或`RECAPTCHA_PROXY_URL=http://127.0.0.1:7022`)

3. 重启`genspark2api`服务。

#### 接入自定义Recaptcha服务

###### 接口：获取令牌

###### 基本信息

- **接口地址**：`/genspark`
- **请求方式**：GET
- **接口描述**：获取用户认证令牌

###### 请求参数

###### 请求头

| 参数名    | 必选 | 类型     | 说明     |
|--------|----|--------|--------|
| cookie | 是  | string | 用户会话凭证 |

###### 响应参数

###### 响应示例

```json
{
  "code": 200,
  "token": "ey********pe"
}
```

## 报错排查

> `Detected Cloudflare Challenge Page`
>

被Cloudflare拦截出5s盾,可配置`PROXY_URL`。

(【推荐方案】[自建ipv6代理池绕过cf对ip的速率限制及5s盾](https://linux.do/t/topic/367413)
或购买[IProyal](https://iproyal.cn/?r=244330))

> `Genspark Service Unavailable`
>
Genspark官方服务不可用,请稍后再试。

> `All cookies are temporarily unavailable.`
>
所有用户(cookie)均到达速率限制,更换用户cookie或稍后再试。

## 生视频请求格式

### Request

**Endpoint**: `POST /v1/videos/generations`

**Content-Type**: `application/json`

#### Request Parameters

| 字段 Field     | 类型 Type | 必填 Required | 描述 Description            | 可选值 Accepted Values                                                                             |
|--------------|---------|-------------|---------------------------|-------------------------------------------------------------------------------------------------|
| model        | string  | 是           | 使用的视频生成模型                 | 模型列表: `sora-2`\|`sora-2-pro`\|`gemini/veo3`\|`gemini/veo3/fast`\|`kling/v2.5-turbo/pro`\|`fal-ai/bytedance/seedance/v1/pro`\|`minimax/hailuo-02/standard`\|`pixverse/v5`\|`fal-ai/bytedance/seedance/v1/lite`\|`gemini/veo2`\|`wan/v2.2`\|`hunyuan`\|`vidu/start-end-to-video`\|`runway/gen4_turbo` |
| aspect_ratio | string  | 是           | 视频宽高比                     | `9:16` \| `16:9` \| `3:4` \|`1:1` \| `4:3`                                                      |
| duration     | int     | 是           | 视频时长（单位：秒）                | 正整数                                                                                             |
| prompt       | string  | 是           | 生成视频的文本描述                 | -                                                                                               |
| auto_prompt  | bool    | 是           | 是否自动优化提示词                 | `true` \| `false`                                                                               |
| image        | string  | 否           | 用于视频生成的基底图片（Base64编码/url） | Base64字符串/url                                                                                   |

---

### Response

#### Response Object

```json
{
  "created": 1677664796,
  "data": [
    {
      "url": "https://example.com/video.mp4"
    }
  ]
}
```

## 其他

**Genspark**(
注册领取1个月Plus): [https://www.genspark.ai](https://www.genspark.ai/invite?invite_code=YjVjMGRkYWVMZmE4YUw5MDc0TDM1ODlMZDYwMzQ4OTJlNmEx)
