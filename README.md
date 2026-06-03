# micro-uac

`micro-uac` 提供一个 HTTP 接口发起 SIP 外呼。呼叫接通后，会读取本地音频文件并通过 RTP 播放给对端。

## 配置文件

项目使用根目录的 [config.json](/Users/liyuhui/Documents/project/micro-uac/config.json)。

当前示例：

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

参数说明：

- `sip.listen_addr`
  SIP 本地监听地址，当前使用 UDP
- `sip.external_ip`
  对外宣告给远端的 SIP/SDP 地址
- `sip.user_agent`
  SIP `User-Agent` 头
- `fs_addr`
  FreeSWITCH 或对端 SIP 地址，格式必须是 `ip:port`
- `log.level`
  日志级别
- `log.file`
  日志文件路径
- `log.also_stdout`
  是否同时输出到标准输出
- `media.default_audio_file`
  默认播放音频文件路径
- `media.default_codec`
  默认编解码，支持 `pcmu` / `pcma`
- `media.default_frame_ms`
  RTP 帧长，单位毫秒
- `http.listen_addr`
  HTTP 接口监听地址

## HTTP 接口

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

字段说明：

- `from`
  主叫 SIP 地址
- `to`
  被叫 SIP 地址
- `request_uri`
  SIP 请求目标地址
- `audio_file`
  播放的本地音频文件
- `codec`
  音频编解码
- `frame_ms`
  RTP 帧长

查询任务：

```http
GET /calls/{call_id}
```

## 测试脚本

测试脚本是 [scripts/create_call.py](/Users/liyuhui/Documents/project/micro-uac/scripts/create_call.py)。

执行方式：

```bash
python3 scripts/create_call.py
```

脚本行为：

- 从 `config.json` 读取 `sip.external_ip`
- 从 `config.json` 读取 `fs_addr`
- 自动构造 `from`、`to`、`request_uri`
- 请求 `http://127.0.0.1:8090/calls`
- 打印请求体和响应体

脚本构造规则：

- `from` -> `<sip:1001@{sip.external_ip}>`
- `to` -> `<sip:1012@{fs_addr.host}>`
- `request_uri` -> `sip:1012@{fs_addr.host}:{fs_addr.port}`

## 音频文件要求

当前支持：

- `.wav`
- `.pcm`

其中 `.wav` 必须满足：

- `8000 Hz`
- `mono`
- `16-bit PCM`
