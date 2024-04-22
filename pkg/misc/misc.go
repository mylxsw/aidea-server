package misc

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/Vernacular-ai/godub/converter"
	"github.com/lithammer/shortuuid/v4"
	"math/rand"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/go-uuid"
	"github.com/mylxsw/asteria/log"

	"gopkg.in/resty.v1"

	"github.com/hashicorp/go-version"
	"github.com/speps/go-hashids/v2"
)

// RestyClient 创建一个失败自动重试的 HTTP 客户端
func RestyClient(retryCount int) *resty.Client {
	return resty.New().
		SetRetryCount(retryCount).
		SetRetryWaitTime(500 * time.Millisecond).
		SetRetryMaxWaitTime(30 * time.Second).
		AddRetryCondition(func(r *resty.Response) (bool, error) {
			statusCode := r.StatusCode()
			return statusCode > 399 && statusCode != 400 && statusCode != 404, nil
		})
}

// MaskPhoneNumber 隐藏手机号码中间四位
func MaskPhoneNumber(phone string) string {
	if len(phone) < 11 {
		return phone
	}

	return phone[:3] + "****" + phone[7:]
}

// MaskStr 隐藏字符串中间部分
func MaskStr(content string, left int) string {
	size := len(content)
	if size < 16 {
		return strings.Repeat("*", size)
	}

	return content[:left] + strings.Repeat("*", size-left*2) + content[size-left:]
}

func HashID(id int64) string {
	hd := hashids.NewData()
	hd.Salt = "aidea is a chat bot for AI, by mylxsw"
	hd.MinLength = 6

	h, _ := hashids.NewWithData(hd)
	e, _ := h.EncodeInt64([]int64{id})

	return e
}

func OrderID(userID int64) string {
	return fmt.Sprintf("%d%.11d", time.Now().UnixNano()-1688000000000000000, userID)
}

// IsChinese 判断是否为中文
func IsChinese(str string) bool {
	if str == "" {
		return false
	}

	var count float64
	for _, v := range str {
		if unicode.Is(unicode.Han, v) {
			count += 1.0
		}
	}

	// 有三分之一以上的字符是中文，则认为是中文
	return count/float64(utf8.RuneCountInString(str)) > 0.3
}

// ContainChinese 判断是否包含中文
func ContainChinese(str string) bool {
	if str == "" {
		return false
	}

	var count float64
	for _, v := range str {
		if unicode.Is(unicode.Han, v) {
			count += 1.0
		}
	}

	return count > 0
}

// WordCount 统计字符串中的字符数
func WordCount(text string) int64 {
	return int64(utf8.RuneCountInString(text))
}

// WordTruncate 截取字符串，如果字符串长度超过 length，则截取 length 个字符
func WordTruncate(text string, length int64) string {
	if WordCount(text) <= length {
		return text
	}

	return string([]rune(text)[:length])
}

// ParseAppleDateTime 解析苹果返回的时间
func ParseAppleDateTime(dt string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05 Etc/GMT", dt)
}

// VersionNewer 比较版本号，当前版本是否比 compareWith 版本新
func VersionNewer(current, compareWith string) bool {
	curVersion, err := version.NewVersion(current)
	if err != nil {
		return false
	}
	compareVersion, err := version.NewVersion(compareWith)
	if err != nil {
		return false
	}

	return curVersion.GreaterThan(compareVersion)
}

// VersionOlder 比较版本号，当前版本是否比 compareWith 版本旧
func VersionOlder(current, compareWith string) bool {
	curVersion, err := version.NewVersion(current)
	if err != nil {
		return false
	}
	compareVersion, err := version.NewVersion(compareWith)
	if err != nil {
		return false
	}

	return curVersion.LessThan(compareVersion)
}

func ResolveAspectRatio(width, height int) string {
	if width == 0 || height == 0 {
		return "1:1"
	}

	gcd := func(a, b int) int {
		if a < b {
			a, b = b, a
		}

		for b != 0 {
			a, b = b, a%b
		}

		return a
	}

	g := gcd(width, height)
	width = width / g
	height = height / g

	return strconv.Itoa(width) + ":" + strconv.Itoa(height)
}

func ResolveHeightFromAspectRatio(width int, aspectRatio string) int {
	segs := strings.SplitN(aspectRatio, ":", 2)
	if len(segs) != 2 {
		return width
	}

	w, _ := strconv.Atoi(segs[0])
	h, _ := strconv.Atoi(segs[1])

	return width * h / w
}

func SubString(str string, length int) string {
	size := utf8.RuneCountInString(str)
	if size <= length {
		return str
	}

	return string([]rune(str)[:length]) + "..."
}

func SubStringRaw(str string, length int) string {
	size := utf8.RuneCountInString(str)
	if size <= length {
		return str
	}

	return string([]rune(str)[:length])
}

// TextSplit 把 text 以 size 个字符为单位分割
func TextSplit(text string, size int) []string {
	var segments []string
	textRunes := []rune(text)
	for i := 0; i < len(textRunes); i += size {
		end := i + size
		if end > len(textRunes) {
			end = len(textRunes)
		}

		segments = append(segments, string(textRunes[i:end]))
	}

	return segments
}

// ImageToRawBase64 把图片转换为 base64 编码
func ImageToRawBase64(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// ImageToBase64Image 把图片转换为 base64 编码图片
func ImageToBase64Image(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	mimeType := http.DetectContentType(data)
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// Base64ImageMediaType 获取 base64 图片的 MIME 类型
func Base64ImageMediaType(base64Image string) (string, error) {
	_, mimeType, err := DecodeBase64ImageWithMime(base64Image)
	if err != nil {
		return "", err
	}

	return mimeType, nil
}

// RemoveImageBase64Prefix 移除 base64 图片的前缀
func RemoveImageBase64Prefix(base64Image string) string {
	return strings.SplitN(base64Image, ",", 2)[1]
}

// AddImageBase64Prefix 添加 base64 图片的前缀
func AddImageBase64Prefix(base64Image, mimeType string) string {
	return "data:" + mimeType + ";base64," + base64Image
}

// ImageToBase64ImageWithMime 把图片转换为 base64 编码图片
func ImageToBase64ImageWithMime(imagePath string) (encoded string, mimeType string, err error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", "", err
	}

	mimeType = http.DetectContentType(data)
	return base64.StdEncoding.EncodeToString(data), mimeType, nil
}

// DecodeBase64Image 解码 base64 图片
func DecodeBase64Image(base64Image string) (data []byte, ext string, err error) {
	// Remove data:image/jpeg;base64, if exist
	d := strings.SplitN(base64Image, ",", 2)
	if len(d) == 2 {
		base64Image = d[1]
	}

	// Decode the base64 image
	decodedData, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		return nil, "", err
	}

	// Detect the content type to get the file extension
	contentType := http.DetectContentType(decodedData)
	exts, _ := mime.ExtensionsByType(contentType)
	if len(exts) > 0 {
		return decodedData, exts[0], nil
	}

	return decodedData, ".png", nil
}

// DecodeBase64ImageWithMime 解码 base64 图片
func DecodeBase64ImageWithMime(base64Image string) (data []byte, mimeType string, err error) {
	// Remove data:image/jpeg;base64, if exist
	d := strings.SplitN(base64Image, ",", 2)
	if len(d) == 2 {
		base64Image = d[1]
	}

	// Decode the base64 image
	decodedData, err := base64.StdEncoding.DecodeString(base64Image)
	if err != nil {
		return nil, "", err
	}

	return decodedData, http.DetectContentType(decodedData), nil
}

// TodayRemainTimeSeconds 获取今日剩余时间
func TodayRemainTimeSeconds() float64 {
	now := time.Now()
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	return endOfDay.Sub(now).Seconds()
}

func NoError(err error) {
	if err != nil {
		log.Warningf("出错啦: %v", err)
	}
}

func NoError2[T any](_ T, err error) {
	if err != nil {
		log.Warningf("出错啦: %v", err)
	}
}

// GenerateAPIToken 生成 API Token
func GenerateAPIToken(name string, uid int64) string {
	randomNumber := rand.Int63n(int64(9999999999))
	return fmt.Sprintf("%s.%d.%x", HashID(uid), time.Now().UnixNano(), sha1.Sum([]byte(fmt.Sprintf("%s:%d:%d:%d", name, uid, time.Now().UnixNano(), randomNumber))))
}

// UUID 生成一个 UUID
func UUID() string {
	ret, _ := uuid.GenerateUUID()
	return ret
}

// ShortUUID 生成一个短 UUID
func ShortUUID() string {
	return shortuuid.New()
}

// PaymentID 生成支付 ID
func PaymentID(userID int64) string {
	return fmt.Sprintf("%s-%s", HashID(userID), shortuuid.New())
}

// FileExt 获取文件扩展名
func FileExt(filename string) string {
	return strings.ToLower(path.Ext(filename))
}

// Sha1 计算 sha1 值
func Sha1(data []byte) string {
	return fmt.Sprintf("%x", sha1.Sum(data))
}

// Md5 计算 md5 值
func Md5(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// WavToMp3 将 wav 文件转换为 mp3 文件
func WavToMp3(wavFileName, mp3FileName string) error {
	w, _ := os.Create(mp3FileName)
	defer w.Close()

	return converter.NewConverter(w).
		WithBitRate(64000).
		WithDstFormat("mp3").
		Convert(wavFileName)
}

// FileSize 获取文件大小
func FileSize(tempPath string) int64 {
	fileInfo, err := os.Stat(tempPath)
	if err != nil {
		return 0
	}

	return fileInfo.Size()
}

// IndexOf Helper function to get the index of T in a slice
func IndexOf[T comparable](slice []T, item T) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	
	return -1
}
