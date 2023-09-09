package controllers

import (
	"fmt"
	"math/rand"

	"github.com/mylxsw/aidea-server/config"
	"github.com/mylxsw/aidea-server/internal/helper"
	"github.com/mylxsw/glacier/infra"

	"github.com/mylxsw/aidea-server/api/auth"
	"github.com/mylxsw/glacier/web"
)

// InfoController 信息控制器
type InfoController struct {
	conf *config.Config `autowire:"@"`
}

// NewInfoController 创建信息控制器
func NewInfoController(resolver infra.Resolver) web.Controller {
	ctl := InfoController{}
	resolver.MustAutoWire(&ctl)
	return &ctl
}

func (ctl *InfoController) Register(router web.Router) {
	router.Group("/info", func(router web.Router) {
		router.Get("/capabilities", ctl.Capabilities)
		router.Get("/version", ctl.Version)
		router.Get("/privacy-policy", ctl.PrivacyPolicy)
		router.Get("/terms-of-user", ctl.TermsOfUser)
		router.Post("/version-check", ctl.VersionCheck)
	})
	router.Group("/share", func(router web.Router) {
		router.Get("/info", ctl.shareInfo)
	})
}

var qrCodes = []string{
	"https://ssl.aicode.cc/ai-server/assets/%E4%BA%8C%E7%BB%B4%E7%A0%81.png",
	"https://ssl.aicode.cc/ai-server/assets/qr-1.png",
	"https://ssl.aicode.cc/ai-server/assets/qr-3.png",
	"https://ssl.aicode.cc/ai-server/assets/qr-4.png",
	"https://ssl.aicode.cc/ai-server/assets/qr-5.png",
	"https://ssl.aicode.cc/ai-server/assets/qr-6.png",
}

func (ctl *InfoController) shareInfo(ctx web.Context, user *auth.UserOptional) web.Response {
	var res = web.M{
		"qr_code": qrCodes[rand.Intn(len(qrCodes))],
		"message": "扫码下载 AIdea，玩转 GPT，实在太有趣啦！",
	}

	if user.User != nil {
		if user.User.InviteCode != "" {
			res["invite_code"] = user.User.InviteCode
			res["message"] = fmt.Sprintf("扫码下载 AIdea，用我的专属邀请码 %s 注册，不仅免费用，还有额外奖励！", user.User.InviteCode)
		}
	}

	return ctx.JSON(res)
}

const CurrentVersion = "1.0.4"

func (ctl *InfoController) VersionCheck(ctx web.Context) web.Response {
	clientVersion := ctx.Input("version")
	clientOS := ctx.Input("os")

	var hasUpdate bool
	if clientOS == "android" || clientOS == "macos" {
		hasUpdate = helper.VersionNewer(CurrentVersion, clientVersion)
	}

	return ctx.JSON(web.M{
		"has_update":     hasUpdate,
		"server_version": CurrentVersion,
		"force_update":   false,
		"url":            "https://aidea.aicode.cc",
		"message":        fmt.Sprintf("新版本 %s 发布啦，赶快去更新吧！", CurrentVersion),
	})
}

// Version 获取版本信息
func (ctl *InfoController) Version(ctx web.Context) web.Response {
	return ctx.JSON(web.M{
		"version": "1.0.1",
	})
}

// Capabilities 获取 AI 平台的能力列表
func (ctl *InfoController) Capabilities(ctx web.Context) web.Response {
	return ctx.JSON(web.M{
		// 是否启用苹果 App 支付
		"applepay-enabled": ctl.conf.EnableApplePay,
		// 是否启用支付宝支付
		"alipay-enabled": ctl.conf.EnableAlipay,
		// 是否启用讯飞星火模型
		"xfyunai-enabled": ctl.conf.EnableXFYunAI,
		// 是否启用百度文心千帆模型
		"baiduwxai-enabled": ctl.conf.EnableBaiduWXAI,
		// 是否启用阿里灵积平台
		"dashscopeai-enabled": ctl.conf.EnableDashScopeAI,
		// 是否启用 OpenAI
		"openai-enabled": ctl.conf.EnableOpenAI,
		// 是否启用翻译功能
		"translate-enabled": ctl.conf.EnableTranslate,
		// 是否启用邮件发送功能
		"mail-enabled": ctl.conf.EnableMail,
	})
}

// PrivacyPolicy 隐私条款
func (ctl *InfoController) PrivacyPolicy(ctx web.Context) web.Response {
	return ctx.HTML(fmt.Sprintf(htmlTemplate, "隐私政策", privacyPolicy))
}

// TermsOfUser 用户协议
func (ctl *InfoController) TermsOfUser(ctx web.Context) web.Response {
	return ctx.HTML(fmt.Sprintf(htmlTemplate, "用户协议", termsOfUser))
}

const htmlTemplate = `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
	<link href="https://cdn.bootcdn.net/ajax/libs/twitter-bootstrap/4.0.0/css/bootstrap.min.css" rel="stylesheet">
    <title>%s</title>
  </head>
  <body><div class="container">%s</div></body>
</html>`

const privacyPolicy = `<h1 id="-">隐私政策</h1>
<p>我们非常重视您的隐私，并致力于保护您在使用我们的应用（以下简称为“本应用”）时提供的个人信息。本隐私政策（以下简称为“本政策”）旨在向您说明我们如何收集、使用、存储和披露您的个人信息。在您开始使用本应用之前，请您仔细阅读并充分理解本政策。您一旦开始使用本应用，即表示您已同意并接受本政策的所有条款和条件。</p>
<h2 id="1-">1. 信息收集</h2>
<p>在您使用本应用时，我们可能会收集以下类型的信息：</p>
<ol>
<li><strong>账户信息</strong>：当您注册本应用的账户时，我们可能会收集您的用户名、密码、电子邮件地址等个人信息。</li>
<li><strong>用户内容</strong>：当您在本应用中创建、生成或分享内容时，我们可能会收集您创作的文本、图片等信息。</li>
<li><strong>通信数据</strong>：当您与本应用的 AI 平台进行聊天时，我们可能会收集您发送的消息以及收到的回复。</li>
<li><strong>设备信息</strong>：我们可能会收集您使用的设备型号、操作系统版本、设备标识符等信息。</li>
<li><strong>日志数据</strong>：我们可能会收集您在使用本应用过程中产生的日志信息，包括访问时间、使用时长、操作记录等。</li>
<li><strong>其他信息</strong>：我们可能会收集您提供给我们的其他个人信息，例如您在与我们沟通时提供的信息。</li>
</ol>
<h2 id="2-">2. 信息使用</h2>
<p>我们可能会将收集的信息用于以下目的：</p>
<ol>
<li>提供、维护和改进本应用的功能和服务；</li>
<li>响应您的咨询和请求；</li>
<li>了解您的需求，以便向您提供更符合您需求的服务；</li>
<li>发现和预防欺诈、滥用和安全问题；</li>
<li>进行市场营销和推广活动，例如向您发送有关本应用的更新和优惠信息。</li>
</ol>
<h2 id="3-">3. 信息共享</h2>
<p>我们不会将您的个人信息出售、出租或与任何第三方分享，除非得到您的明确许可，或以下情况之一：</p>
<ol>
<li>为了遵守法律法规、响应政府部门和法院的要求；</li>
<li>为了保护本公司、本应用及其用户的权益、财产和安全；</li>
<li>在本公司合并、收购或资产出售的情况下，我们可能会将您的个人信息转让给相关第三方。</li>
</ol>
<h2 id="4-">4. 信息存储</h2>
<p>我们会采取合理的技术和管理措施来保护您的个人信息，防止您的个人信息遭到未经授权的访问、披露、使用、修改或丢失。但请注意，尽管我们已经竭尽全力保护您的个人信息，但任何安全系统都不能保证100%的安全。</p>
<h2 id="5-">5. 信息修改和删除</h2>
<p>您可以在本应用中查看、修改您的个人信息。如果您想删除您的账户或个人信息，请联系我们。在收到您的请求后，我们将尽快处理，并根据相关法律法规的要求删除您的个人信息。</p>
<h2 id="6-">6. 儿童隐私</h2>
<p>本应用不针对13岁以下的儿童。我们不会故意收集儿童的个人信息。如果您是13岁以下的儿童，请不要使用本应用。如果您是13岁以下儿童的父母或监护人，并发现您的孩子向我们提供了个人信息，请联系我们，我们将尽快采取措施删除相关信息。</p>
<h2 id="7-">7. 隐私政策的变更</h2>
<p>我们可能会不时更新本政策。在本政策发生重大变更时，我们会通过本应用或发送邮件等方式通知您。请您定期查看本政策，以了解我们如何保护您的个人信息。您继续使用本应用的行为将被视为您同意并接受经修订后的本政策。</p>
<h2 id="8-">8. 联系我们</h2>
<p>如果您对本政策或我们的隐私实践有任何疑问、建议或投诉，请随时与我们联系。您可以通过以下方式联系我们：</p>
<ul>
<li>电子邮件：<a href="mailto:mylxsw@aicode.cc">mylxsw@aicode.cc</a></li>
</ul>
<p>感谢您选择使用本应用。请您放心，我们会尽最大努力保护您的个人信息安全。</p>
<p>最后更新日期：2023年5月23日</p>
`

const termsOfUser = `<h1 id="-">用户协议</h1>
<h2 id="1-">1. 概述</h2>
<p>欢迎使用我们的应用（以下简称为“本应用”）。本应用由我们（以下简称为“本公司”）开发并提供。在使用本应用之前，请您仔细阅读并充分理解本用户协议（以下简称为“本协议”）。您一旦开始使用本应用，即表示您已同意并接受本协议的所有条款和条件。</p>
<p>本协议描述了您与本公司之间关于本应用使用的权利和义务。如果您不同意本协议的任何部分，请您立即停止使用本应用。</p>
<h2 id="2-">2. 账户</h2>
<p>为使用本应用，您需要注册一个账户。在注册过程中，您必须提供真实、准确、完整和最新的个人信息。您有责任保护您的账户安全，并对您账户下的所有活动承担责任。如果您发现未经授权的账户使用，您应立即通知本公司。</p>
<h2 id="3-">3. 授权</h2>
<p>在您遵守本协议的前提下，本公司授予您在个人设备上使用本应用的有限、非排他性、不可转让的许可。您同意不会出于任何目的复制、修改、分发、出售、租赁、对外授权或进行逆向工程本应用。</p>
<h2 id="4-">4. 使用规则</h2>
<p>在使用本应用时，您同意遵守以下规则：</p>
<ol>
<li>不发布、传播、存储任何违反国家法律法规、社会公共利益、公序良俗的内容；</li>
<li>不侵犯他人知识产权、商业秘密等合法权益；</li>
<li>不从事任何可能对本应用正常运行造成不利影响的行为；</li>
<li>不利用本应用进行任何违法犯罪活动。</li>
</ol>
<h2 id="5-">5. 内容产权</h2>
<p>本应用允许用户生成和分享原创内容。您保留您创作的内容的知识产权，但您在此授予本公司针对您创作的内容的全球范围内的免费、非独家、可转让、可许可的许可，以便本公司可以使用、复制、修改、分发、展示和传播该等内容。</p>
<h2 id="6-">6. 免责声明</h2>
<p>本应用按照现状提供，本公司不对本应用的适用性、可靠性、准确性、完整性和有效性做任何明示或暗示的保证。您使用本应用产生的风险自行承担。</p>
<p>本公司不对因使用或无法使用本应用所导致的任何直接、间接、附带、特殊、惩罚性或后果性损失承担责任。</p>
<h2 id="7-">7. 终止</h2>
<p>本公司保留在任何时候因任何原因终止您使用本应用的权利，包括但不限于您违反了本协议的规定。在终止您的使用权后，您应立即销毁本应用的所有副本。</p>
<h2 id="8-">8. 修改</h2>
<p>本公司保留随时修改本协议的权利。修改后的协议将在本应用内公布。您继续使用本应用将视为您接受修改后的协议。</p>
<h2 id="9-">9. 法律适用和管辖</h2>
<p>本协议的订立、执行和解释及争议的解决均适用中华人民共和国法律。如双方就本协议内容或执行发生争议，应首先尽量友好协商解决；协商不成时，任何一方均有权将争议提交至本公司所在地人民法院诉讼解决。</p>
<h2 id="10-">10. 其他</h2>
<p>本协议构成您与本公司之间就本应用使用达成的完整协议，取代您和本公司先前就本应用达成的任何口头或书面协议。本协议的任何规定被认定为无效、不可执行或非法，不应影响其他规定的有效性和可执行性。</p>
<p>如您对本协议有任何疑问，请联系本公司。</p>
`
