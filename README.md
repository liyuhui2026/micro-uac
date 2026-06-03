# micro-uac

`micro-uac` 是一个用于 SIP 外呼调试的简单 UAC 服务。

它提供一个 HTTP 接口创建外呼任务，呼叫接通后读取本地音频文件，编码为 RTP 并发送给对端。

## 功能概览

- 通过 HTTP API 发起外呼
- 使用 SIP 建立会话
- 使用 RTP 发送本地音频
- 支持 `.wav` / `.pcm` 音频输入
- 支持本地脚本测试外呼
- 支持 Docker、docker-compose、Kubernetes 部署

## 目录结构

- `cmd/server`
  HTTP 服务入口
- `internal/sip`
  SIP 建链与通话 dialog 管理
- `internal/rtp`
  RTP 发送逻辑
- `internal/media`
  音频文件读取与编码
- `internal/service`
  通话主流程控制
- `internal/task`
  外呼任务管理
- `audio/`
  本地音频目录
- `.runtime/`
  运行时目录，保存日志、PID、构建产物
- `scripts/create_call.py`
  测试外呼脚本
- `start_server.sh`
  本地后台启动脚本
- `Dockerfile`
  容器镜像构建文件
- `docker-compose.yml`
  本地容器编排文件
- `k8s/`
  Kubernetes 部署清单

## 配置文件

项目使用根目录的 [config.json](/Users/liyuhui/Documents/project/micro-uac/config.json)。

当前配置示例：

```json
{
  "sip": {
    "listen_addr": "0.0.0.0:5061",
    "external_ip": "10.103.2.34",
    "user_agent": "micro-uac/1.0"
  },
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

关键字段说明：

- `sip.listen_addr`
  SIP 本地监听地址，当前服务使用 UDP
- `sip.external_ip`
  对外宣告给远端的 SIP/SDP 地址
- `http.listen_addr`
  HTTP API 监听地址
- `media.default_audio_file`
  默认播放音频文件路径
- `media.default_codec`
  默认编解码，当前支持 `pcmu` / `pcma`
- `media.default_frame_ms`
  RTP 帧长，默认 `20ms`
- `log.file`
  结构化日志文件输出路径

## 音频要求

当前支持：

- `.wav`
- `.pcm`

其中 `.wav` 必须满足：

- `8000 Hz`
- `mono`
- `16-bit PCM`

默认音频文件是 [audio/demo.wav](/Users/liyuhui/Documents/project/micro-uac/audio/demo.wav)。

如果你有 MP3 文件，可以这样转码：

```bash
ffmpeg -y -i audio/test_demo.mp3 -ac 1 -ar 8000 -sample_fmt s16 audio/demo.wav
```

## 本地启动

使用后台启动脚本：

```bash
./start_server.sh
```

脚本行为：

1. 检查 `.runtime/micro-uac-server.pid`
2. 如果存在旧 PID，则尝试结束旧进程
3. 执行 `go build ./...`
4. 重建 `./.runtime`
5. 编译服务端到 `./.runtime/micro-uac-server`
6. 后台启动服务

运行产物：

- PID 文件
  [\.runtime/micro-uac-server.pid](/Users/liyuhui/Documents/project/micro-uac/.runtime/micro-uac-server.pid)
- 启动输出
  [\.runtime/stdout.log](/Users/liyuhui/Documents/project/micro-uac/.runtime/stdout.log)
- 程序日志
  [\.runtime/micro-uac.log](/Users/liyuhui/Documents/project/micro-uac/.runtime/micro-uac.log)

停止服务示例：

```bash
kill "$(cat .runtime/micro-uac-server.pid)"
```

## 发起测试外呼

项目内置测试脚本 [scripts/create_call.py](/Users/liyuhui/Documents/project/micro-uac/scripts/create_call.py)：

```bash
python3 scripts/create_call.py
```

脚本会打印：

- 请求 URL
- 请求体
- 响应状态码
- 响应体

当前脚本默认参数：

- caller: `1001`
- called: `1012`
- request_uri: `sip:1012@10.103.2.34:5060`
- audio_file: `audio/demo.wav`
- codec: `pcmu`

## HTTP API

创建外呼：

```http
POST /calls
Content-Type: application/json
```

请求体示例：

```json
{
  "from": "<sip:1001@10.103.2.34>",
  "to": "<sip:1012@10.103.2.34>",
  "request_uri": "sip:1012@10.103.2.34:5060",
  "audio_file": "audio/demo.wav",
  "codec": "pcmu",
  "frame_ms": 20
}
```

查询任务：

```http
GET /calls/{call_id}
```

## 日志排障

主要日志文件：

- [\.runtime/micro-uac.log](/Users/liyuhui/Documents/project/micro-uac/.runtime/micro-uac.log)
- [\.runtime/stdout.log](/Users/liyuhui/Documents/project/micro-uac/.runtime/stdout.log)

常见排查方式：

```bash
tail -n 300 .runtime/micro-uac.log
tail -n 200 .runtime/stdout.log
```

当前日志会记录：

- 发出的 `INVITE`
- 收到的 SIP 响应
- 收到的 `BYE`
- 发出的 `BYE`
- `BYE` 对应收到的响应
- RTP 开始发送
- RTP 帧发送进度

## 手动构建与测试

构建整个项目：

```bash
go build ./...
```

单独构建服务：

```bash
go build -o .runtime/micro-uac-server ./cmd/server
```

运行全部测试：

```bash
go test ./...
```

只跑 `internal`：

```bash
go test ./internal/...
```

校验 Python 脚本语法：

```bash
python3 -m py_compile scripts/create_call.py
```

## Docker

镜像构建命令：

```bash
docker build --platform linux/amd64 -t micro-uac:latest .
```

说明：

- Dockerfile 构建阶段已设置：
  - `GOPROXY=https://goproxy.cn,direct`
  - `GOSUMDB=sum.golang.google.cn`
- 构建目标平台是 `linux/amd64`
- 运行镜像内已包含 `python3`
- 镜像内可直接执行 `/app/scripts/create_call.py`

## docker-compose

项目提供了 [docker-compose.yml](/Users/liyuhui/Documents/project/micro-uac/docker-compose.yml)。

启动方式：

```bash
docker compose up -d
```

挂载内容：

- `./.runtime -> /app/.runtime`
- `./audio -> /app/audio`
- `./config.json -> /app/config.json`

说明：

- `docker-compose.yml` 直接使用本地镜像 `micro-uac:latest`
- 不会在 `docker compose up` 时重新构建镜像
- 网络模式使用 `host`
- 因为使用 `host` 网络，不再单独做端口映射

## Kubernetes

项目提供了：

- [k8s/configmap.yaml](/Users/liyuhui/Documents/project/micro-uac/k8s/configmap.yaml)
- [k8s/deployment.yaml](/Users/liyuhui/Documents/project/micro-uac/k8s/deployment.yaml)

应用方式：

```bash
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
```

说明：

- `ConfigMap` 提供 `config.template.json`
- `Deployment` 使用镜像 `micro-uac:latest`
- 模板配置挂载到 `/app/config-template`
- 最终配置文件写入 `/app/config/config.json`
- `config` 与 `runtime` 目录都挂载为 `emptyDir`
- `Deployment` 使用 Downward API 注入 `POD_IP`
- 容器启动时会生成最终 `/app/config/config.json`
- 容器暴露：
  - `8090/TCP`
  - `5061/UDP`

注意：

- 当前 `k8s` 清单还没有包含 `Service`
- `sip.external_ip` 在 K8s 中会动态替换为当前 Pod IP
- 如果要真正部署到集群外部可访问环境，通常还需要继续补：
  - `Service`
  - `NodePort` / `LoadBalancer`
  - 或 `hostNetwork`
  - 并确认 FreeSWITCH 或对端网络确实能访问 Pod IP

## 网络受限环境

如果本机访问官方 Go 服务受限，可执行：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.google.cn
```
