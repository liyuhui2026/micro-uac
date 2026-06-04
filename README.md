# micro-uac

`micro-uac` 是一个基于 Go 的轻量 SIP 外呼服务。

它提供一个 HTTP 接口用于创建呼叫，在通话建立后读取本地音频文件，转码为 G.711 `PCMU` 或 `PCMA`，并通过 RTP 推流给对端。

## 功能概览

- 通过 HTTP API 创建外呼任务
- 支持 SIP `INVITE` 发起呼叫
- 支持本地 `.wav` / `.pcm` 音频文件播放
- 支持 `pcmu` / `pcma`
- 支持 CLI 直接发起单次呼叫
- 提供 Python 测试脚本
- 提供 Dockerfile 和 Kubernetes 清单

当前任务管理器一次只允许一个呼叫任务运行；当已有任务执行中，再次创建任务会返回 `409 Conflict`。

## 项目结构

- `cmd/server`
  HTTP 服务入口
- `cmd/cli`
  命令行拨号入口
- `internal/httpapi`
  HTTP 路由与请求处理
- `internal/service`
  呼叫流程编排
- `internal/sip`
  SIP 客户端
- `internal/rtp`
  RTP 发送
- `internal/media`
  音频读取与 G.711 编码
- `scripts/create_call.py`
  调用 HTTP API 的测试脚本
- `k8s/`
  Kubernetes 部署文件

## 环境要求

- Go `1.26`
- 可访问的 SIP 对端或 FreeSWITCH
- 可被对端回连的 RTP 地址

如果使用 `.wav` 文件，必须满足以下格式：

- `8000 Hz`
- `mono`
- `16-bit PCM`

`.pcm` 文件默认按 `16-bit little-endian mono 8000 Hz` 原始 PCM 处理。

## 配置文件

默认配置文件是项目根目录的 [config.json](/Users/liyuhui/Documents/project/micro-uac/config.json)。

示例：

```json
{
  "sip": {
    "listen_addr": "0.0.0.0:5061",
    "external_ip": "10.103.2.34",
    "user_agent": "micro-uac/1.0"
  },
  "fs_addr": "10.103.2.34:5060",
  "log": {
    "level": "info",
    "file": ".runtime/micro-uac.log",
    "also_stdout": false
  },
  "media": {
    "default_audio_file": "audio/demo.wav",
    "default_codec": "pcmu",
    "default_frame_ms": 20
  },
  "http": {
    "listen_addr": "0.0.0.0:8090"
  }
}
```

字段说明：

- `sip.listen_addr`
  本地 SIP 监听地址，格式如 `0.0.0.0:5061`
- `sip.external_ip`
  写入 SDP 的对外地址。`sip.listen_addr` 使用 `0.0.0.0` 时必须显式配置
- `sip.user_agent`
  SIP `User-Agent`
- `fs_addr`
  默认 SIP 发送目标地址，格式必须是 `host:port`
- `log.level`
  日志级别
- `log.file`
  日志文件路径
- `log.also_stdout`
  是否同时输出到标准输出
- `media.default_audio_file`
  默认音频文件路径
- `media.default_codec`
  默认编码，支持 `pcmu` / `pcma`
- `media.default_frame_ms`
  默认 RTP 帧长，单位毫秒
- `http.listen_addr`
  HTTP 服务监听地址

## 本地运行

启动 HTTP 服务：

```bash
go run ./cmd/server -config config.json
```

或先构建再运行：

```bash
go build -o .runtime/micro-uac-server ./cmd/server
./.runtime/micro-uac-server -config config.json
```

仓库内也提供了一个本地启动脚本：

```bash
./start_server.sh
```

这个脚本会：

- 编译项目
- 输出服务二进制到 `.runtime/micro-uac-server`
- 启动后台进程
- 将 PID 写入 `.runtime/micro-uac-server.pid`
- 将标准输出写入 `.runtime/stdout.log`

## CLI 直接拨号

命令行入口在 `cmd/cli`，适合不经过 HTTP，直接发起一次呼叫：

```bash
go run ./cmd/cli \
  -config config.json \
  -from '<sip:1001@10.103.2.34>' \
  -to '<sip:1012@10.103.2.34>' \
  -request-uri 'sip:1012@10.103.2.35:5080' \
  -fs-addr '10.103.2.34:5060' \
  -line-addr '10.103.2.35:5080' \
  -target-uri 'sip:2001@10.103.2.35:5080' \
  -audio-file 'audio/demo.wav' \
  -codec pcmu \
  -frame-ms 20
```

其中：

- `from`
  SIP `From` 头
- `to`
  SIP `To` 头
- `request-uri`
  SIP 请求 URI
- `fs-addr`
  实际发包目标地址
- `line-addr`
  线路地址，用于构造 `X-Sip-Client-Target-Uri`
- `target-uri`
  目标 SIP URI；其用户部分用于 `X-Sip-Client-Target-Uri`

如果未显式传入以下参数，会按配置或请求内容自动补齐：

- `fs_addr` 默认取 `config.json` 的 `fs_addr`
- `line_addr` 默认从 `request_uri` 中提取 `host:port`
- `target_uri` 默认等于 `request_uri`
- `audio_file` 默认取 `media.default_audio_file`
- `codec` 默认取 `media.default_codec`
- `frame_ms` 默认取 `media.default_frame_ms`

## HTTP API

### 创建呼叫

```http
POST /calls
Content-Type: application/json
```

请求体示例：

```json
{
  "from": "<sip:1001@10.103.2.34>",
  "to": "<sip:1012@10.103.2.34>",
  "request_uri": "sip:1012@10.103.2.35:5080",
  "fs_addr": "10.103.2.34:5060",
  "line_addr": "10.103.2.35:5080",
  "target_uri": "sip:2001@10.103.2.35:5080",
  "audio_file": "audio/demo.wav",
  "codec": "pcmu",
  "frame_ms": 20
}
```

最小请求体：

```json
{
  "from": "<sip:1001@10.103.2.34>",
  "to": "<sip:1012@10.103.2.34>",
  "request_uri": "sip:1012@10.103.2.35:5080"
}
```

成功时返回 `202 Accepted`：

```json
{
  "call_id": "8ec85c4e-1e56-4e5b-b2f5-1d0e65fa3f1d",
  "state": "created",
  "started_at": "2026-06-04T12:00:00Z"
}
```

常见失败：

- `400 Bad Request`
  JSON 非法
- `409 Conflict`
  当前已有任务执行中
- `500 Internal Server Error`
  呼叫创建流程异常

### 查询呼叫状态

```http
GET /calls/{call_id}
```

成功时返回 `200 OK`：

```json
{
  "call_id": "8ec85c4e-1e56-4e5b-b2f5-1d0e65fa3f1d",
  "sip_call_id": "f84f0dc3-4f9c-4daa-a9a4-938f6d4f6d34",
  "state": "completed",
  "started_at": "2026-06-04T12:00:00Z",
  "ended_at": "2026-06-04T12:00:08Z"
}
```

呼叫状态包括：

- `created`
- `dialing`
- `ringing`
- `answered`
- `streaming`
- `terminating`
- `completed`
- `failed`

### curl 示例

```bash
curl -X POST http://127.0.0.1:8090/calls \
  -H 'Content-Type: application/json' \
  -d '{
    "from": "<sip:1001@10.103.2.34>",
    "to": "<sip:1012@10.103.2.34>",
    "request_uri": "sip:1012@10.103.2.35:5080",
    "fs_addr": "10.103.2.34:5060",
    "line_addr": "10.103.2.35:5080",
    "target_uri": "sip:2001@10.103.2.35:5080",
    "audio_file": "audio/demo.wav",
    "codec": "pcmu",
    "frame_ms": 20
  }'
```

## Python 测试脚本

测试脚本是 [scripts/create_call.py](/Users/liyuhui/Documents/project/micro-uac/scripts/create_call.py)。

它会读取配置文件中的：

- `sip.external_ip`
- `fs_addr`
- `http.listen_addr`

然后自动构造请求并发送到 `POST /calls`。

基础用法：

```bash
python3 scripts/create_call.py
```

指定参数：

```bash
python3 scripts/create_call.py \
  --from-number 031186778297 \
  --to-number 9200711 \
  --target-number 13729020276 \
  --fs-addr 10.167.128.24:5060 \
  --line-addr icc.icsoc.net:5066
```

脚本会打印：

- request URL
- request body
- response status
- response body

以上命令会构造出类似下面的请求体：

```json
{
  "from": "<sip:031186778297@10.103.2.34>",
  "to": "<sip:9200711@10.167.128.24>",
  "request_uri": "sip:9200711@icc.icsoc.net:5066",
  "fs_addr": "10.167.128.24:5060",
  "line_addr": "icc.icsoc.net:5066",
  "target_uri": "sip:13729020276@icc.icsoc.net:5066",
  "audio_file": "audio/demo.wav",
  "codec": "pcmu",
  "frame_ms": 20
}
```

其中 `from` 里的 IP 来自 `config.json` 中的 `sip.external_ip`。如果你的实际环境不是 `10.103.2.34`，这里会按你的配置值变化。

对应的 `INVITE` 示例：

```sip
INVITE sip:9200711@icc.icsoc.net:5066 SIP/2.0
Via: SIP/2.0/UDP 10.103.2.34:5061;branch=z9hG4bK-<auto-branch>
From: <sip:031186778297@10.103.2.34>;tag=<auto-tag>
To: <sip:9200711@10.167.128.24:5060>
Call-ID: <auto-call-id>
CSeq: 1 INVITE
Contact: <sip:10.103.2.34:5061>
Max-Forwards: 70
User-Agent: micro-uac/1.0
X-Sip-Client-Target-Uri: sip:13729020276@icc.icsoc.net:5066
Content-Type: application/sdp
Content-Length: <auto-length>

v=0
o=- 0 0 IN IP4 10.103.2.34
s=micro-uac
c=IN IP4 10.103.2.34
t=0 0
m=audio <auto-rtp-port> RTP/AVP 0
a=rtpmap:0 PCMU/8000
a=sendonly
```

字段对应关系：

- `Request-URI` -> `sip:9200711@icc.icsoc.net:5066`
- `To` -> `<sip:9200711@10.167.128.24:5060>`
- `X-Sip-Client-Target-Uri` -> `sip:13729020276@icc.icsoc.net:5066`
- SIP 实际发送目标 -> `10.167.128.24:5060`
- SDP IP -> `config.json` 中的 `sip.external_ip`
- RTP 端口 -> 运行时动态分配

## SIP 头处理规则

当前实现相对普通 UAC 有以下约定：

- `To` 头中的地址部分会使用 `fs_addr`
- 额外写入 `X-Sip-Client-Target-Uri`
- `X-Sip-Client-Target-Uri` 的用户部分来自 `target_uri`
- `X-Sip-Client-Target-Uri` 的主机和端口来自 `line_addr`

例如请求：

```json
{
  "to": "<sip:1012@10.103.2.34>",
  "request_uri": "sip:1012@10.103.2.35:5080",
  "fs_addr": "10.103.2.34:5060",
  "line_addr": "10.103.2.35:5080",
  "target_uri": "sip:2001@10.200.0.8:5090"
}
```

发出的关键头语义为：

- `To` 里的用户部分取自 `to`
- `To` 里的地址部分取自 `fs_addr`
- `X-Sip-Client-Target-Uri` 的用户部分取自 `target_uri`
- `X-Sip-Client-Target-Uri` 的地址部分取自 `line_addr`

## Docker

构建镜像：

```bash
docker build -t micro-uac:local .
```

运行容器：

```bash
docker run --rm \
  -p 8090:8090/tcp \
  -p 5061:5061/udp \
  -v "$(pwd)/config.json:/app/config.json:ro" \
  -v "$(pwd)/audio:/app/audio:ro" \
  micro-uac:local
```

容器默认启动命令：

```bash
/app/micro-uac-server -config /app/config.json
```

## Kubernetes

仓库中已有以下清单：

- [k8s/configmap.yaml](/Users/liyuhui/Documents/project/micro-uac/k8s/configmap.yaml)
- [k8s/deployment.yaml](/Users/liyuhui/Documents/project/micro-uac/k8s/deployment.yaml)

部署命令：

```bash
kubectl apply -f k8s/
```

如果使用指定 namespace：

```bash
kubectl apply -n <namespace> -f k8s/
```

当前 `Deployment` 使用了私有镜像拉取密钥：

```yaml
imagePullSecrets:
  - name: imagepullsecret-rte-sec
```

如果集群中尚未创建，需要先创建：

```bash
kubectl create secret docker-registry imagepullsecret-rte-sec \
  --docker-server=hub-master.agoralab.co \
  --docker-username=<username> \
  --docker-password=<password> \
  --docker-email=<email>
```

部署后检查：

```bash
kubectl get pods
kubectl describe deploy micro-uac
kubectl logs deploy/micro-uac
kubectl rollout status deploy/micro-uac
```

当前仓库还没有 `Service` 清单。如果需要暴露 `8090/TCP` 或 `5061/UDP`，需要额外补一个 `Service`。

## 测试

运行单元测试：

```bash
go test ./...
```

## 排障建议

- 启动失败先检查 `config.json` 格式和 `fs_addr` 是否为 `host:port`
- 对端无音频先检查 `sip.external_ip` 是否可被对端访问
- `.wav` 播放失败先确认是否为 `8000 Hz mono 16-bit PCM`
- 创建呼叫返回 `409` 说明已有任务正在运行
- HTTP 可用但呼叫失败时，优先查看日志文件和 `.runtime/stdout.log`
