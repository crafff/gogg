package main

import (
	// "encoding/json"
	"fmt"
	"io"
	// "log"
	"net/http"
	"github.com/bytedance/sonic"
)

type SimpleClient struct {
	httpClient	*http.Client
	apiKey		string
}


func NewSimpleClient(apiKey string) *SimpleClient {
	return &SimpleClient{
		httpClient: &http.Client{},
		apiKey: apiKey,   // 参数分行书写最后一个要待逗号
	}
}

// CLOSE_WAIT
// 如果服务器主动发FIN，Go没有用resp.Body.Close()
// 连接会卡在CLOSE_WAIT状态
// 每个CLOSE_WATI会占用一个FD，linux默认一个进程只能开1024个FD

// 关于服务器主动发FIN
// 服务器不可能维持数量过多的“死连接”，不然会占用大量内存
// 超过IdleTimeout, 服务器会发FIN，资源回收
// 只有本地没有Close才会变TIME_OUT
// 否则Go会自动回收

// 如歌处理服务器主动发FIN（已发送）
// 1. 连接回收：从连接池踢出
// 2. 重试：发起请求正好关了，换一个连接重试
// 3. 主动关闭，客户端在服务器IdletTimeout之前主动关闭连接

// KEEP_ALIVE
// 微服务之间可能有几百个请求，只要两个请求之间的时间间隔小于服务器的IdleTimeout
// 这个连接就会被一直复用，不会出发服务器自动挂断
// 如果连接很久没动静，内核会发嗅探包，如果对方还在就保持连接，否则就认为断了 

// 防御慢速连接攻击
// 1. 应用层-设置精细的超时： ReadHeaderTimeout（只握手不发数据），IdleTimeout（空闲太久）
// 2. 传输层-限制单 IP 的最大连接数：golang.org/x/net/netutil 限制并发
// 3. 网络层：利用“空闲探测” (TCP Keep-Alive) tcp_keepalive_time: 探测前的闲置时间（默认 2 小时，建议改为 5-10 分钟）。tcp_keepalive_probes: 探测失败几次才断开。
// 4. 架构层：反向代理（Nginx/网关）
// 5. 业务逻辑：识别并拉黑 （拉黑发送大量无效请求的IP）

// TCP滑动窗口和各种缓冲区
// 1. 服务器发送缓冲区：服务器Write先进入发送缓冲区，如果包丢了重传，只有当服务器接收到ACK之后，数据才会从缓冲区删除
// 2. TCP窗口：在不需要等待ACK下，空中能飞多少数据，如果内核缓冲区满了，窗口最缩小到0，如果内核缓冲区空了，窗口会变大
// 3. 内核缓冲区：内核管理的接收数据的缓冲区，会影响TCP滑动窗口的大小
// 4. Go缓冲区：Go程序所占用的内存，1)这是你在代码里定义的变量，比如 buf := make([]byte, 1024)，2)或者是 bufio.NewReader 内部维护的 4KB 空间。
	// Go 通过发送系统调用（Syscall）把数据从内存缓冲区拷贝到Go缓冲区（变量中）
// 背压：Go逻辑处理太慢 -> Go缓冲区不更新 -> Read不执行 -> 内核缓冲区满 -> TCP窗口变小 -> 服务器发送速度变慢
// 优化：会调大 Linux 的 rmem_max (内核接收缓冲区最大值)，就是为了在 Go 程序还没来得及 Read 时，让内核能多囤点货，减少网络往返的等待。

// 滑动窗口的调整过程
// 载体：TCP Header中的Windows Size
// 1. 初始窗口：TCP握手阶段告知（比如64KB）
// 2. 服务器没发一个包，窗口可用空间减少，如果发了64KB（窗口大小）还没收到回信，停止
// 3. 接收端处理完一部分数据（Goc程序调用了Read)内核接收缓冲区腾出空间，内核发送ACK包更新Windows Size（比如从0变回16KB）
	// 发送端收到这个ACK，向右移动窗口，获得了新的发送配额

// 关于error
// 创建一个错误
// 1. 静态错误（不需要动态参数）： var ErrNotFound = errors.New("资源不存在")
// 2. 动态错误：err := fmt.Errorf("用户 ID %d 验证失败", userID)
// 处理错误
// res, err := Func()
// if err != nil
// // 自定义错误
// type MyError struct {
// 	Status	int
// 	Message	string
// 	Err		error
// }

// // 必须实现Error()方法
// func (e *MyError) Error() string {
// 	return fmt.Sprintf("err %d %s: %v", e.Status, e.Message, e.Err)
// }

// // 用于errors.Is/As
// func (e *MyError) UnWarp() error {
// 	return e.Err
// }

// errors.Is errors.As
// err := GetRiotAccount()

// // 1. 检查是不是网络超时（值判定）
// if errors.Is(err, context.DeadlineExceeded) {
//     fmt.Println("请求超时了")
// }

// // 2. 检查是不是业务层面的错误，并拿到 API 返回的错误码（类型与数据提取）
// var apiErr *RiotApiError
// if errors.As(err, &apiErr) {
//     fmt.Printf("Riot API 报错，状态码: %d, 信息: %s\n", apiErr.StatusCode, apiErr.Message)
// }

	
	
type UserResponse struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

type UserInfo struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

func (c *SimpleClient) GetPuuid(gameName string, tagLine string) (string, error) {
	// 网址可变部分的string的定义
	region := "americas"

	// 最终request网址的生成，用fmt.Springtf
	url := fmt.Sprintf("https://%s.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s?api_key=%s",
		region, gameName, tagLine, c.apiKey)

	// http.Response: 重要的fields: StatusCode int, Header Header(map[string][]string), Body io.ReadCloser
	// ContentLength int64
	// resp只要收到Header就会返回，不需要收到整个Body
	resp, err := c.httpClient.Get(url)  // Get只有url这一个参数，返回*http.Response,和err
	if err != nil {
		// 网络层面的错误
		// 1. 域名解析失败 （访问一个不存在的域名）
		// 2. 连接超时，对方服务器太忙或IP不同，三次握手没握上
		// 3. 连接被拒绝 （目标端口没开，或者被防火墙拦截）
		// 4. TLS握手失败： 证书到期，证书不匹配或加密协议对不上
		// 5. 中途断网：数据传输一半，网线拔了或者或者WIFI断了
		// 6. 触发了Client.Timeout： 设置了5秒超时，6秒还没传完数据
		// log.Fatal(err) 只要哦实现了Error() string都是error，log.Fatal(err)打印错误信息并终止程序
		return "", fmt.Errorf("GetPuuid(): httpClient.Get Fail, url: %s, err: %s", url, err)
	}
	// 只要err == nil, resp就一定不为nil, 必须要手动关闭resp.Body，否则会一直占用TCP连接
	// Close要在err ！= nil之后，否则resp可能是nil，会触发空指针
	defer resp.Body.Close()

	// 检查StatusCode
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("Server returned error: %d", resp.StatusCode)
		}
		msg, _ := sonic.Get(bodyBytes, "status", "message")
		msgStr, _ := msg.String()
		return "", fmt.Errorf("Server returned error: %d, message: %s", resp.StatusCode, msgStr)
	}

	// Body是Stream，边传边读
	// 拿到resp之后Body的开头部分已经抵达内核缓冲区（依据TCP滑动窗口大小）

	// 关于Body的使用
	// 1. 只读不关：连接无法复用，下次还得握手，慢慢耗尽FD
	// 2. 只关不读：连接物理断开，无法服复用，下次还得握手
	// 3. 不关不读：占用FD，内核内存，CLOSE_WAIT堆积，业务瘫痪
	// 4. 读完并关：连接回到池子

	// 如果Body不读不能直接Close，要先丢弃Body里面的内容
	// 丢弃 Body 里的内容，确保连接能被放回池子复用
	// io.Copy(io.Discard, resp.Body) 
	// resp.Body.Close()

	// 处理Body是依然要处理err
	// body, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	// 传输中途失败
	// 	log.Fatal(err)
	// }

	var result UserResponse
	sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result)
	
	// sonic.Decoder().Decode()

	return result.Puuid, nil
}

func (sc *SimpleClient) GetPuuidHeader(gameName string, tagline string) (*UserInfo, error) {
	// create header
	region := "americas"
	url := fmt.Sprintf("https://%s.api.riotgames.com/riot/account/v1/accounts/by-riot-id/%s/%s?api_key=%s",
			region, gameName, tagline, sc.apiKey)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("GetPuuidHeader(): new request fail: %w", err)
	}

	req.Header.Set("X-Riot-Token", sc.apiKey)
	
	resp, err := sc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GetPuuidHeader(): Do request fail: %w", err)
	}
	
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("GetPuuidHeader(): Riotapi returned error") 
		}
		msg, _ := sonic.Get(bodyBytes, "status", "message")
		msgStr, _ := msg.String() 
		return nil, fmt.Errorf("GetPuuidHeader(): Riotapi returned error, message: %s", msgStr)
	}

	var result UserInfo
	sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result) 
	return &result, nil
}