# AIDEA 外部应用支付 API（v1）

公开地址：`https://<AIDEA API 域名>/partner/v1/payments/docs`。文档随服务端版本发布，支付功能未启用时仍可阅读。所有金额单位为人民币分；接口只面向应用服务端，密钥不能放在 macOS 安装包中。

## 接入准备

1. 向 AIDEA 管理员申请唯一 `client_id`。应用服务端生成 Ed25519 PKCS8 PEM 私钥，提交对应的 PEM 公钥；AIDEA 将公钥、金额上下限写入 `external_pay_clients`。
2. AIDEA 的通知公钥可从 `GET /partner/v1/payments/public-key` 获取。接入方应保存或固定该公钥并在换钥时更新。
3. 选择 `alipay_page`（浏览器打开 `pay_url`）或 `wechat_native`（将 `pay_url` 作为二维码内容展示）。
4. 每笔订单提交自己的 `callback_url`，必须是公网 HTTPS 地址、443 端口；AIDEA 不要求预先配置回调地址。

不同应用建议各用一个 `client_id` 和密钥，方便单独停用和限定金额；同一应用可以为不同订单提交不同回调地址。AIDEA 的微信、支付宝商户密钥不会交给接入应用。

生成密钥示例：

```bash
openssl genpkey -algorithm ED25519 -out client-private.pem
openssl pkey -in client-private.pem -pubout -out client-public.pem
```

管理员登记应用的示例 SQL（把 `client-public.pem` 的完整内容作为参数绑定，勿拼接 SQL）：

```sql
INSERT INTO external_pay_clients
 (client_id, public_key_pem, enabled, allowed_channels, min_amount_fen, max_amount_fen, max_daily_orders)
 VALUES (?, ?, 1, 'alipay_page,wechat_native', 1, 10000000, 1000);
```

## 请求签名

每次调用都在 `Authorization: Bearer <JWT>` 中发送 EdDSA 签名 JWT。JWT 是签名格式，**不是加密格式**；全程使用 HTTPS。JWT Header 固定为 `{"alg":"EdDSA","typ":"JWT"}`，Payload 示例：

```json
{
  "iss":"my-macos-app",
  "aud":"aidea-external-pay",
  "iat":1792300000,
  "nbf":1792300000,
  "exp":1792300060,
  "jti":"每次请求唯一的随机值",
  "method":"POST",
  "path":"/partner/v1/payments",
  "body_sha256":"请求体原始字节 SHA-256 的无填充 Base64URL 编码"
}
```

`path` 是完整 Request URI，包括查询字符串；GET 请求使用空字节串的 SHA-256。`exp-iat` 最长 120 秒，服务器时钟允许约 30 秒误差。`jti` 不能重复，重试请求需重新签名并使用新 `jti`。应用服务端应保存订单号，用相同 `merchant_order_id` 重试下单。

Node.js 18+ 请求示例（私钥只放在应用服务端）：

```js
import { createHash, randomUUID, sign } from 'node:crypto';
import { readFileSync } from 'node:fs';

const path = '/partner/v1/payments';
const body = Buffer.from(JSON.stringify({
  merchant_order_id: 'order-20261018-001',
  channel: 'wechat_native',
  amount_fen: 1990,
  currency: 'CNY',
  subject: '示例服务一个月',
  callback_url: 'https://example.com/api/payment/aidea-notify'
}));
const now = Math.floor(Date.now() / 1000);
const header = Buffer.from(JSON.stringify({ alg: 'EdDSA', typ: 'JWT' })).toString('base64url');
const payload = Buffer.from(JSON.stringify({
  iss: 'my-macos-app', aud: 'aidea-external-pay',
  iat: now, nbf: now, exp: now + 60, jti: randomUUID(),
  method: 'POST', path,
  body_sha256: createHash('sha256').update(body).digest('base64url')
})).toString('base64url');
const signed = `${header}.${payload}`;
const privateKey = readFileSync('/secure/client-private.pem');
const jwt = `${signed}.${sign(null, Buffer.from(signed), privateKey).toString('base64url')}`;
const response = await fetch(`https://<AIDEA API 域名>${path}`, {
  method: 'POST',
  headers: { Authorization: `Bearer ${jwt}`, 'Content-Type': 'application/json' },
  body
});
console.log(response.status, await response.json());
```

## 创建支付单

`POST /partner/v1/payments`，请求体：

```json
{
  "merchant_order_id":"order-20261018-001",
  "channel":"wechat_native",
  "amount_fen":1990,
  "currency":"CNY",
  "subject":"示例服务一个月",
  "callback_url":"https://example.com/api/payment/aidea-notify"
}
```

返回 `201`：

```json
{
  "payment_id":"EX_...",
  "client_id":"my-macos-app",
  "merchant_order_id":"order-20261018-001",
  "channel":"wechat_native",
  "amount_fen":1990,
  "currency":"CNY",
  "subject":"示例服务一个月",
  "status":"pending",
  "pay_url":"weixin://wxpay/...",
  "created_at":"2026-10-18T12:00:00Z",
  "updated_at":"2026-10-18T12:00:00Z"
}
```

相同应用的相同 `merchant_order_id` 再次提交相同参数，返回原订单（`200`）；关键参数不同返回 `409 order_conflict`。若支付渠道临时不可用，返回 `502 provider_error`，可稍后用原 `merchant_order_id` 重试。`pay_url` 对微信是二维码内容，对支付宝是 AIDEA 的付款跳转地址。该地址只在订单待支付且创建后 15 分钟内可打开；支付宝收银台也设置了 15 分钟付款期限。不要将支付地址误认为已付款证明。

支付宝付款后会跳转到 AIDEA 的返回页。页面会尝试自动关闭；若浏览器不允许，用户可关闭标签页并回到应用。此跳转只用于引导用户，不能据此发放权益；应用仍应等待支付通知或查询到 `paid`。

## 查询和补漏

- `GET /partner/v1/payments/{payment_id}`：查询自己的订单。
- `GET /partner/v1/payments?cursor=<上次返回的游标>&limit=100`：按更新时间、记录 ID 顺序读取自己的订单；返回 `{"orders":[...],"next_cursor":"...","has_more":false}`。有结果时始终保存 `next_cursor`，`has_more=true` 时继续翻页；`false` 时下次定期从保存的游标继续。
- `POST /partner/v1/payments/{payment_id}/close`：关闭自己的待支付订单；AIDEA 先向渠道查单。有待支付交易时向渠道关单；支付宝确认交易尚不存在时，直接关闭本地订单并禁用付款跳转地址。已支付订单返回 `409`。

订单状态：`creating`、`pending`、`paid`、`closed`、`init_failed`。只有 `paid` 表示 AIDEA 已经确认收款。若因网络或回调丢失导致暂时 `pending`，AIDEA 会主动向渠道查单；应用也应继续查询而非立刻生成第二笔付款。

每笔支付的微信或支付宝订单号由 AIDEA 生成。AIDEA 会核对渠道回调中的签名、订单号、商户身份、金额和币种，支付成功后更新订单并创建通知事件。

## AIDEA 向应用发送支付通知

AIDEA 以 `POST` 请求下单时提交的 `callback_url`，`Content-Type: application/jwt`，请求体为 EdDSA 签名 JWT。使用 AIDEA 公钥验证。通知 Payload 示例：

```json
{
  "iss":"aidea-external-pay",
  "aud":"my-macos-app",
  "iat":1792300000,
  "nbf":1792300000,
  "exp":1792300120,
  "jti":"WH_...",
  "event_id":"EV_...",
  "payment_id":"EX_...",
  "merchant_order_id":"order-20261018-001",
  "channel":"wechat_native",
  "amount_fen":1990,
  "currency":"CNY",
  "status":"paid"
}
```

接收方必须验证签名、`iss`、`aud`、有效期和金额，按 `event_id` 或 `payment_id` 幂等处理；建议再调用单笔查询接口核实，然后开通权益。返回任意 2xx 表示成功接收；非 2xx 或网络错误会自动重试。通知可能重复，也可能比用户返回页面晚到。应用端应通过分页查询补漏。

Node.js 18+ 验签示例（`rawJWT` 是请求体原始文本）：

```js
import { verify } from 'node:crypto';
import { readFileSync } from 'node:fs';

function verifyAideaNotice(rawJWT, expectedOrder) {
  const parts = rawJWT.split('.');
  if (parts.length !== 3) throw new Error('invalid JWT');
  const header = JSON.parse(Buffer.from(parts[0], 'base64url'));
  if (header.alg !== 'EdDSA' || header.typ !== 'JWT') throw new Error('invalid algorithm');
  const signed = `${parts[0]}.${parts[1]}`;
  const publicKey = readFileSync('/secure/aidea-public.pem');
  if (!verify(null, Buffer.from(signed), publicKey, Buffer.from(parts[2], 'base64url'))) {
    throw new Error('invalid signature');
  }
  const event = JSON.parse(Buffer.from(parts[1], 'base64url'));
  const now = Math.floor(Date.now() / 1000);
  if (![event.iat, event.nbf, event.exp, event.amount_fen].every(Number.isSafeInteger) ||
      !event.event_id || !event.jti) throw new Error('missing notification claims');
  if (event.iss !== 'aidea-external-pay' || event.aud !== 'my-macos-app' ||
      event.iat > now + 30 || event.nbf > now + 30 || event.exp < now ||
      event.exp - event.iat > 120 ||
      event.status !== 'paid' || event.payment_id !== expectedOrder.paymentId ||
      event.merchant_order_id !== expectedOrder.merchantOrderId ||
      event.amount_fen !== expectedOrder.amountFen || event.currency !== 'CNY' ||
      event.channel !== expectedOrder.channel) {
    throw new Error('notification does not match order');
  }
  return event;
}
```

通知处理顺序：验签和检查声明 → 按应用订单号核对金额 → 在应用自己的数据库事务里记录 `event_id`、标记已支付并发放权益 → 提交事务后返回 2xx。如果落库失败，返回非 2xx 让 AIDEA 重试。通知最多自动尝试 20 次，随后进入 `dead` 状态；应监控 `external_pay_events` 中的 `dead` 和未完成记录，修复故障后由运维将对应事件设为 `pending`、`attempts=0`、`next_attempt_at=CURRENT_TIMESTAMP(6)`。补漏查询仍可发现已支付订单。

## 错误与限制

错误统一为 `{"code":"...","message":"..."}`。常见状态码：400 参数错误，401 签名或身份错误，403 渠道未授权，404 订单不存在或不属于当前应用，409 同一业务订单参数冲突，429 达到日订单上限，502 渠道暂不可用。请求体上限 32 KiB。回调地址只允许公网 HTTPS 443，拒绝跳转和内网目标。

外部支付 API 不处理 AIDEA 点数充值；应用自己负责权益发放、售后与退款流程。退款接口尚未开放，请通过 AIDEA 商户运营流程处理，并在应用侧同步退款记录。AIDEA 的订单列表用于两套系统补漏；财务对账仍须核对微信、支付宝的渠道账单。

## 服务端部署

`aidea-server.yaml` 需配置 `external-pay-enabled: true`、`external-pay-base-url: https://<AIDEA API 域名>`、`external-pay-private-key-path: /data/etc/...`。AIDEA 自己的通知签名私钥可用 `openssl genpkey -algorithm ED25519 -out /data/etc/aidea-external-pay.pem` 生成，设置 `chmod 600` 并确保服务进程可读。它与每个接入应用的私钥是不同的密钥。启用前运行数据库迁移、登记至少一个接入应用，确保 `enable-scheduler: true`，并配置 Nginx 转发 `/partner/v1/payments` 到 AIDEA Server。对外文档和公钥端点无需认证；业务接口需要上述 JWT。渠道回调端点仅接受经过微信或支付宝验证的通知。建议先用小额订单分别验证两种渠道的支付、通知、重复回调和补漏查询，再扩大接入范围。
