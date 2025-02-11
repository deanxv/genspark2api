<p align="right">
   <strong>中文</strong> 
</p>
<div align="center">

# Genspark2API

_觉得有点意思的话 别忘了点个 ⭐_

<a href="https://t.me/+LGKwlC_xa-E5ZDk9">
    <img src="https://telegram.org/img/website_icon.svg" width="16" height="16" style="vertical-align: middle;">
    <span style="text-decoration: none; font-size: 12px; color: #0088cc; vertical-align: middle;">Telegram 交流群</span>
</a>

<sup><i>(原`coze-discord-proxy`交流群, 此项目仍可进此群**交流** / **反馈bug**)</i></sup>

</div>



## 功能

- [x] 支持对话接口(流式/非流式)(`/chat/completions`)(请求非以下列表的模型会触发`Mixture-of-Agents`模式)
    - **gpt-4o**
    - **o1**
    - **o3-mini-high**
    - **claude-3-5-sonnet**
    - **claude-3-5-haiku**
    - **gemini-2.0-flash**
    - **deep-seek-v3**
    - **deep-seek-r1**
- [x] 支持**联网搜索**,在模型名后添加`-search`即可(如:`gpt-4o-search`)
- [x] 支持识别**图片**/**文件**多轮对话
- [x] 支持文生图接口(`/images/generations`)
    - **flux**
    - **flux-speed**
    - **flux-pro/ultra**
    - **ideogram**
    - **recraft-v3**
    - **dall-e-3**
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

如果上面的镜像无法拉取,可以尝试使用 GitHub 的 Docker 镜像,将上面的`deanxv/genspark2api`替换为`ghcr.io/deanxv/genspark2api`即可。

### 部署到第三方平台

<details>
<summary><strong>部署到 Zeabur</strong></summary>
<div>

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
8. `AUTO_MODEL_CHAT_MAP_TYPE=1`  [可选]自动配置Model绑定Chat(默认:1)[0:关闭,1:开启]
9. `MODEL_CHAT_MAP=claude-3-5-sonnet=a649******00fa,gpt-4o=su74******47hd`  [可选]Model绑定Chat(多个请以,分隔),详细请看[进阶配置](#解决模型自动切换导致降智问题)
10. `ROUTE_PREFIX=hf`  [可选]路由前缀,默认为空,添加该变量后的接口示例:`/hf/v1/chat/completions`

~~11. `YES_CAPTCHA_CLIENT_KEY=******`  [可选]YesCaptcha Client Key 过谷歌验证,详细请看[使用YesCaptcha过谷歌验证](#使用YesCaptcha过谷歌验证)~~

~~12. `SESSION_IMAGE_CHAT_MAP=aed9196b-********-4ed6e32f7e4d=0c6785e6-********-7ff6e5a2a29c,aefwer6b-********-casds22=fda234-********-sfaw123`  [可选]Session绑定Image-Chat(多个请以,分隔),详细请看[进阶配置](#生图模型配置)~~

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
2. 选择需要绑定的对话的模型(示例:`claude-3-5-sonnet`),发起对话。
3. 点击ask请求,此时最上方url中的`id`(或响应中的`id`)即为此对话唯一id。
   ![img.png](docs/img4.png)
4. 配置环境变量 `MODEL_CHAT_MAP=claude-3-5-sonnet=3cdcc******474c5` (多个请以,分隔)

### 生图模型配置[**暂不需要**]

> 配置环境变量 SESSION_IMAGE_CHAT_MAP

~~1. 打开**F12**开发者工具。~~
~~2. 选择生成图像,选择任一生图模型,发起对话。~~
~~3. 点击ask请求,此时最上方url中的`id`(或响应中的`id`)即为此对话唯一id,然后在请求头中获取`session_id`的值。
   ![img.png](docs/img7.png)~~
~~4. 配置环境变量 `SESSION_IMAGE_CHAT_MAP=aed9196b-********-4ed6e32f7e4d=0c6785e6-********-7ff6e5a2a29c` (即session=chatId的格式,多个请以,分隔)~~

### 使用YesCaptcha过谷歌验证[**暂不需要**]

> genspark官方目前文生图接口需要过谷歌验证,可使用YesCaptcha解决。
>
> **tip**: 过一次谷歌验证消耗20积分,约**0.0167元人民币**(1元人民币约能用60次)。


~~1. 注册 [YesCaptcha](https://yescaptcha.com/i/021iAE)[此链接注册直达**vip5**]~~

~~2. 获取`Client Key`~~
~~![img.png](docs/img6.png)~~

~~3. 配置环变量`YES_CAPTCHA_CLIENT_KEY=******`~~

~~4. 重启服务~~

## 报错排查

> `Detected Cloudflare Challenge Page`
>

被Cloudflare拦截出5s盾,可配置`PROXY_URL`。

(【推荐方案】[自建ipv6代理池绕过cf对ip的速率限制及5s盾](https://linux.do/t/topic/367413)或购买[IProyal](https://iproyal.cn/?r=244330))

> `Genspark Service Unavailable`
>
Genspark官方服务不可用,请稍后再试。

> `All cookies are temporarily unavailable.`
>
所有用户(cookie)均到达速率限制,更换用户cookie或稍后再试。

## 其他

**Genspark**(
注册领取1个月Plus): [https://www.genspark.ai](https://www.genspark.ai/invite?invite_code=YjVjMGRkYWVMZmE4YUw5MDc0TDM1ODlMZDYwMzQ4OTJlNmEx)
