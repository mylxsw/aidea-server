# 支付宝在线支付配置教程

## 支付宝商家平台

支付宝商家平台： https://b.alipay.com/

在支付宝商家平台开通以下服务

- APP 支付
- 手机网站支付
- 电脑网站支付

![](https://ssl.aicode.cc/mweb/16957843150892.jpg)

## 支付宝开放平台

开放平台地址：https://open.alipay.com/

安卓 APP 信息如下

- 应用签名 6d8ce4fe64934f062d445ec78f6a4082
- 应用包名 cc.aicode.flutter.askaide.askaide

1. 控制中心-创建移动应用
    ![](https://ssl.aicode.cc/mweb/16957844790413.jpg)
2. 填写应用信息
    ![](https://ssl.aicode.cc/mweb/16957849751549.jpg)
   ![](https://ssl.aicode.cc/mweb/16957854300644.jpg)

3. 开发设置，设置密钥加签方式
    ![](https://ssl.aicode.cc/mweb/16957850449046.jpg)
4. 下载密钥到本地
    ![](https://ssl.aicode.cc/mweb/16957850632285.jpg)
5. 绑定支付产品
    ![](https://ssl.aicode.cc/mweb/16957850769504.jpg)



## 配置服务端

配置文件配置以下内容

```yaml
######## 支付宝配置 ########

enable-alipay: true
alipay-appid: "2021004101024050"
alipay-app-private-key: /data/aidea-server/certs/alipay-app-private-key.txt
alipay-app-public-key: /data/aidea-server/certs/appCertPublicKey_2021004101024050.crt
alipay-root-cert: /data/aidea-server/certs/alipayRootCert.crt
alipay-public-key: /data/aidea-server/certs/alipayCertPublicKey_RSA2.crt
alipay-notify-url: https://your_domain/v1/payment/callback/alipay-notify
alipay-return-url: https://your_domain/public/payment/alipay-return
```
