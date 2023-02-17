# TLSProbe

TLS证书时间收集器，用于告警。

特性：
- 和指定地址的TCP端口进行TLS握手，获取证书时间信息。
- 扫描指定主机的所有端口，如果TLS握手成功，获取证书时间信息。
- 从域名服务商获取所有A、CNAME的解析记录。并进行TLS探测，拿到证书时间信息。目前支持的域名服务商：
  - 阿里云
  - DNSPod
- 支持动态reload配置文件。

## 功能
### TLSChecker

每次访问`metrics`接口，`TLSProbe`会尝试与所有在配置文件配置的`TLSCheckers`中的地址进行TLS握手。并返回以下`metrics`信息：
```text
tls_checker{CertDNSNames="[*.example.com example.com]",NotAfter="2023-05-28 23:59:59 +0000 UTC",NotBefore="2022-05-17 00:00:00 +0000 UTC",domain="abc.exmpale.com",error="",host="12.34.45.78",port="443"} 1
tls_checker{CertDNSNames="[*.example.com example.com]",NotAfter="2023-05-28 23:59:59 +0000 UTC",NotBefore="2022-05-17 00:00:00 +0000 UTC",domain="foo.exmpale.com",error="",host="12.34.45.78",port="443"} 1
```
每个label的解析：
- `CertDNSNames`: TLS证书的CN
- `NotAfter`: TLS证书的`NotAfter`
- `NotBefore`: TLS证书的`NotBefore`
- `domain`: 进行TLS握手时，Client Hello 中SNI处填写的域名。
- `host`: tcp主机地址。
- `port`: tcp的端口。

同时为了方便计算证书到期时间，提供了下面两个指标
tls证书中的`NotAfter`
```text
tls_checker_not_after{domain="abc.example.com",host="12.34.45.78",port="443"} 1.684108799e+09
tls_checker_not_after{domain="foo.example.com",host="12.34.45.78",port="443"} 1.684108799e+09
```
tls证书中的`NotBefore`
```text
tls_checker_not_before{domain="abc.example.com",host="12.34.45.78",port="443"} 1.6504992e+09
tls_checker_not_before{domain="foo.example.com",host="12.34.45.78",port="443"} 1.6504992e+09
```

#### Prometheus 告警规则配置
```text
time() - (last_over_time(tls_checker_not_after[4h]) + on(domain, port, host) group_right() last_over_time(tls_checker[4h])) > -2.592e+06
```
查询符合30天之内即将到期的地址。

### hostScanner

在配置文件当中配置对指定host进行tls端口探测，保存探测成功的端口号。
并生成`TLSChecker`，当每次访问`metrics`接口时回获取最新的证书时间信息。

### autoDiscover

用于生成hostScanner，目前支持从DNS服务商获取DNS解析记录，每条A或CName解析记录会生成一个`hostScanner`。
目前支持的DNSProvider：
- 阿里云
- DNSPod

配置示例：
```yaml
autoDiscover:
- name: yunpian-aliyun
  type: AliDNS
  options:
    accessKeyId: ""
    accessKeySecret: ""
- name: qipeng-dnspod
  type: DNSPod
  options:
    secretId: ""
    secretKey: ""
```

## 其他配置
### maxConnections

所有的`hostScanner`共用的并发池。用于控制和所有端口进行握手的并发池。

### maxConnectConnections

当`metrics`接口被请求时，会对所有创建的`TLSChecker`中的地址进行握手，获取最新的证书信息。
该参数用于控制最大的请求数。

配置示例：
```yaml
autoDiscover:
- name: yunpian-aliyun
  type: AliDNS
  options:
    accessKeyId: ""
    accessKeySecret: ""
- name: qipeng-dnspod
  type: DNSPod
  options:
    secretId: ""
    secretKey: ""
hostScannersConfig:
  - host: baidu.com
    TLSOptions:
      timeout: 3000
  - host: google.com
    TLSOptions:
      timeout: 3000
  - host: github.com
    TLSOptions:
      timeout: 3000
      skipVerify: true
listenAddr: "127.0.0.1:9217"
maxConnections: 1500
maxConnectConnections: 1500
```

### metrics 接口地址

默认为：`/metrics`