<p align="right">
   <strong><a href="README.md">中文</a> | English</strong> 
</p>
<div align="center">

# Genspark2API

_If you find this interesting, don't forget to give it a ⭐_

<a href="https://t.me/+LGKwlC_xa-E5ZDk9">
  <img src="https://img.shields.io/badge/Telegram-AI Wave Community-0088cc?style=for-the-badge&logo=telegram&logoColor=white" alt="Telegram Community" />
</a>

<sup><i>AI Wave Community</i></sup> · <sup><i>(Provides free APIs and AI bots)</i></sup>

---

**Genspark2API** is a high-performance proxy server written in Go that wraps **Genspark.ai** capabilities into **OpenAI-compatible API endpoints**.

### 🌟 Core Features

- 🤖 **Conversational AI Models**: Support for GPT-5.1 series, Claude Opus/Sonnet 4.5, Gemini 3, O3-Pro and other mainstream models
- 🖼️ **Text-to-Image**: Support for DALL·E, Flux, Imagen4 and various drawing models
- 🎬 **Text/Image-to-Video**: Support for Sora-2, Veo3, Kling, Runway and other video generation models
- 🔄 **OpenAI API Compatible**: Direct integration with any OpenAI-compatible clients/middleware
- 🛡️ **Smart Protection Bypass**: Built-in Cloudflare and ReCaptcha v3 handling logic (requires configuration per documentation)
- 🔀 **Multi-Cookie Pool Rotation**: Automatic load distribution across multiple accounts with failover retry
- 🔧 **Tool Calling Support**: Support for OpenAI-style function calling (can be disabled via `TOOL_CALLING_ENABLED=0`)
- 🌐 **Web Search**: Add `-search` suffix to model name to enable web search (e.g., `gpt-5.1-search`)
- 📊 **Streaming Response**: SSE streaming output with experience identical to official OpenAI API

### 📦 Quick Start

```bash
docker run -d --name genspark2api \
  -p 7055:7055 \
  -e GS_COOKIE="your_genspark_cookie" \
  -e API_SECRET="your_api_secret" \
  deanxv/genspark2api:latest
```

</div>

> ⚠️ **Important**: Genspark officially enforces `ReCaptchaV3` validation. 
> Failure to pass may cause model degradation or image generation issues. Please refer to [genspark-playwright-proxy V3 verification service](#genspark-playwright-prxoy服务过V3验证) and configure the environment variable `RECAPTCHA_PROXY_URL`.

## Features

- [x] Support for chat interface (streaming/non-streaming) (`/chat/completions`) (requesting models not in the following list will trigger `Mixture-of-Agents` mode)
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
- [x] Support for **web search** by adding `-search` suffix to model name (e.g., `gpt-4o-search`)
- [x] Support for **image**/**file** recognition in multi-turn conversations
- [x] Support for text-to-image interface (`/images/generations`)
    - **fal-ai/nano-banana**
    - **fal-ai/bytedance/seedream/v4**
    - **gpt-image-1**
    - **flux-pro/ultra**
    - **flux-pro/kontext/pro**
    - **imagen4**
- [x] Support for text/image-to-video interface (`/videos/generations`), see [Video Generation Request Format](#生视频请求格式)
- [x] Support for custom request header validation (Authorization)
- [x] Support for cookie pool (random)
- [x] Support for automatic cookie switching retry on request failure (requires cookie pool configuration)
- [x] Configurable automatic chat record deletion
- [x] Configurable proxy requests (environment variable `PROXY_URL`)
- [x] Configurable Model-Chat binding (solves model auto-switching causing **intelligence degradation**), see [Advanced Configuration](#解决模型自动切换导致降智问题)

### API Documentation:

Omitted

### Example:

<span><img src="docs/img2.png" width="800"/></span>

## How to Use

Omitted

## How to Integrate with NextChat

Fill in the interface address (ip:port/domain) and API-Key (`PROXY_SECRET`), everything else can be filled randomly.

> If you haven't built your own NextChat panel, here's one that's already set up: [NeatChat](https://ai.aytsao.cn/)

<span><img src="docs/img5.png" width="800"/></span>

## How to Integrate with one-api

Fill in the `BaseURL` (ip:port/domain) and key (`PROXY_SECRET`), everything else can be filled randomly.

<span><img src="docs/img3.png" width="800"/></span>

## Deployment

### Deploy using Docker-Compose (All In One)

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
      - GS_COOKIE=******  # cookie (multiple separated by commas)
      - API_SECRET=123456  # [Optional] Interface key - modify this line for request header validation value (multiple separated by commas)
      - TZ=Asia/Shanghai
```

### Deploy using Docker

```docker
docker run --name genspark2api -d --restart always \
-p 7055:7055 \
-v $(pwd)/data:/app/genspark2api/data \
-e GS_COOKIE=***** \
-e API_SECRET="123456" \
-e TZ=Asia/Shanghai \
deanxv/genspark2api
```

Replace `API_SECRET` and `GS_COOKIE` with your own values.

If the above image cannot be pulled, try using the GitHub Docker image by replacing `deanxv/genspark2api` with `ghcr.io/deanxv/genspark2api`.

### Deploy to Third-Party Platforms

<details>
<summary><strong>Deploy to Zeabur</strong></summary>
<div>

[![Deployed on Zeabur](https://zeabur.com/deployed-on-zeabur-dark.svg)](https://zeabur.com?referralCode=deanxv&utm_source=deanxv)

> Zeabur's servers are overseas, automatically solving network issues, ~~and the free quota is sufficient for personal use~~

1. First **fork** a copy of the code.
2. Go to [Zeabur](https://zeabur.com?referralCode=deanxv), log in with GitHub, enter the console.
3. In Service -> Add Service, select Git (first-time use requires authorization), select your forked repository.
4. Deploy will start automatically, cancel it first.
5. Add environment variables

   `GS_COOKIE:******`  cookie (multiple separated by commas)

   `API_SECRET:123456` [Optional] Interface key - modify this line for request header validation value (multiple separated by commas) (same usage as openai-API-KEY)

Save.

6. Select Redeploy.

</div>

</details>

<details>
<summary><strong>Deploy to Render</strong></summary>
<div>

> Render provides free quota, and you can further increase the quota by binding a card

Render can deploy docker images directly without forking the repository: [Render](https://dashboard.render.com)

</div>
</details>

## Configuration

### Environment Variables

1. `PORT=7055` [Optional] Port, default is 7055
2. `DEBUG=true` [Optional] DEBUG mode, can print more information [true: on, false: off]
3. `API_SECRET=123456` [Optional] Interface key - modify this line for request header (Authorization) validation value (same as API-KEY) (multiple separated by commas)
4. `GS_COOKIE=******` cookie (multiple separated by commas)
5. `AUTO_DEL_CHAT=0` [Optional] Automatically delete chat after completion (default: 0) [0: off, 1: on]
6. `REQUEST_RATE_LIMIT=60` [Optional] Single IP request rate limit per minute, default: 60 times/min
7. `PROXY_URL=http://127.0.0.1:10801` [Optional] Proxy
8. `RECAPTCHA_PROXY_URL=http://127.0.0.1:7022` [Optional] genspark-playwright-proxy verification service address, just fill in the domain or ip:port. (Example: `RECAPTCHA_PROXY_URL=https://genspark-playwright-proxy.com` or `RECAPTCHA_PROXY_URL=http://127.0.0.1:7022`), see [genspark-playwright-proxy V3 verification service](#genspark-playwright-prxoy服务过V3验证)
9. `AUTO_MODEL_CHAT_MAP_TYPE=1` [Optional] Automatically configure Model-Chat binding (default: 1) [0: off, 1: on]
10. `MODEL_CHAT_MAP=claude-3-7-sonnet=a649******00fa,gpt-4o=su74******47hd` [Optional] Model-Chat binding (multiple separated by commas), see [Advanced Configuration](#解决模型自动切换导致降智问题)
11. `ROUTE_PREFIX=hf` [Optional] Route prefix, default is empty, interface example after adding this variable: `/hf/v1/chat/completions`
12. `RATE_LIMIT_COOKIE_LOCK_DURATION=600` [Optional] Cookie disable time when rate limit is reached, default is 600s
13. `REASONING_HIDE=0` [Optional] **Hide** reasoning process (default: 0) [0: off, 1: on]

~~14. `SESSION_IMAGE_CHAT_MAP=aed9196b-********-4ed6e32f7e4d=0c6785e6-********-7ff6e5a2a29c,aefwer6b-********-casds22=fda234-********-sfaw123` [Optional] Session-Image-Chat binding (multiple separated by commas), see [Advanced Configuration](#生图模型配置)~~

~~15. `YES_CAPTCHA_CLIENT_KEY=******` [Optional] YesCaptcha Client Key for Google verification, see [Using YesCaptcha for Google Verification](#使用YesCaptcha过谷歌验证)~~

### How to Get Cookies

1. Open **F12** developer tools.
2. Initiate a conversation.
3. Click the ask request, the **cookie** in the request header is the value needed for the environment variable **GS_COOKIE**.

> **Note:** The `session_id=f9c60******cb6d` part is required, other content is optional, i.e., environment variable `GS_COOKIE=session_id=f9c60******cb6d`

![img.png](docs/img.png)

## Advanced Configuration

### Solving Model Auto-Switching Causing Intelligence Degradation

#### Method 1 (This configuration is enabled by default) [Recommended]

> Configure environment variable **AUTO_MODEL_CHAT_MAP_TYPE=1**
>
> Under this configuration, the conversation ID will be obtained when calling the model and bind the model.

#### Method 2

> Configure environment variable MODEL_CHAT_MAP
>
> **Purpose:** Specify conversations to solve model auto-switching causing intelligence degradation.

1. Open **F12** developer tools.
2. Select the model for the conversation you want to bind (example: `claude-3-7-sonnet`), initiate a conversation.
3. Click the ask request, the `id` in the top URL (or `id` in the response) is the unique ID for this conversation.
   ![img.png](docs/img4.png)
4. Configure environment variable `MODEL_CHAT_MAP=claude-3-7-sonnet=3cdcc******474c5` (multiple separated by commas)

### genspark-playwright-proxy Service for V3 Verification

1. Deploy genspark-playwright-proxy with docker

#### docker

```docker 
docker run --name genspark-playwright-proxy -d --restart always \
-p 7022:7022 \
-v $(pwd)/data:/app/genspark-playwright-proxy/data \
-e PROXY_URL=http://account:pwd@ip:port #  [Optional] Recommended residential dynamic proxy, configuring proxy increases verification success rate but slows response.
-e TZ=Asia/Shanghai \
deanxv/genspark-playwright-proxy
```

#### docker-compose

```docker-compose
version: '3.4'

services:
  genspark-playwright-proxy:
    image: deanxv/genspark-playwright-proxy:latest
    container_name: genspark-playwright-proxy
    restart: always
    ports:
      - "7022:7022"
    volumes:
      - ./data:/app/genspark-playwright-proxy/data
    environment:
      - PROXY_URL=http://account:pwd@ip:port #  [Optional] Recommended residential dynamic proxy, configuring proxy increases verification success rate but slows response.
```

2. After deployment, configure `genspark2api` environment variable `RECAPTCHA_PROXY_URL`, just fill in the domain or ip:port. (Example: `RECAPTCHA_PROXY_URL=https://genspark-playwright-proxy.com` or `RECAPTCHA_PROXY_URL=http://127.0.0.1:7022`)

3. Restart `genspark2api` service.

#### Integrating Custom Recaptcha Service

###### API: Get Token

###### Basic Information

- **API Endpoint**: `/genspark`
- **Request Method**: GET
- **API Description**: Get user authentication token

###### Request Parameters

###### Request Headers

| Parameter | Required | Type   | Description      |
|-----------|----------|--------|------------------|
| cookie    | Yes      | string | User session credentials |

###### Response Parameters

###### Response Example

```json
{
  "code": 200,
  "token": "ey********pe"
}
```

## Error Troubleshooting

> `Detected Cloudflare Challenge Page`

Blocked by Cloudflare 5s shield, configure `PROXY_URL`.

([Recommended solution] [Build your own ipv6 proxy pool to bypass cf IP rate limits and 5s shield](https://linux.do/t/topic/367413) or purchase [IProyal](https://iproyal.cn/?r=244330))

> `Genspark Service Unavailable`

Genspark official service is unavailable, please try again later.

> `All cookies are temporarily unavailable.`

All users (cookies) have reached the rate limit, change user cookies or try again later.

## Video Generation Request Format

### Request

**Endpoint**: `POST /v1/videos/generations`

**Content-Type**: `application/json`

#### Request Parameters

| Field        | Type   | Required | Description                      | Accepted Values                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
|--------------|--------|----------|----------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| model        | string | Yes      | Video generation model to use     | Model list: `sora-2`\|`sora-2-pro`\|`gemini/veo3`\|`gemini/veo3/fast`\|`kling/v2.5-turbo/pro`\|`fal-ai/bytedance/seedance/v1/pro`\|`minimax/hailuo-02/standard`\|`pixverse/v5`\|`fal-ai/bytedance/seedance/v1/lite`\|`gemini/veo2`\|`wan/v2.2`\|`hunyuan`\|`vidu/start-end-to-video`\|`runway/gen4_turbo` |
| aspect_ratio | string | Yes      | Video aspect ratio               | `9:16` \| `16:9` \| `3:4` \|`1:1` \| `4:3`                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| duration     | int    | Yes      | Video duration (in seconds)      | Positive integer                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| prompt       | string | Yes      | Text description for video generation | -                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| auto_prompt  | bool   | Yes      | Whether to auto-optimize prompt  | `true` \| `false`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| image        | string | No       | Base image for video generation (Base64/url) | Base64 string/url                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |

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

## Other

**Genspark** (Register to get 1 month Plus): [https://www.genspark.ai](https://www.genspark.ai/invite?invite_code=YjVjMGRkYWVMZmE4YUw5MDc0TDM1ODlMZDYwMzQ4OTJlNmEx)
