package lightsocks

import (
	"log"
	"net"
)

// LsLocal 客户端类
type LsLocal struct {
	Cipher     *cipher // 密码类
	ListenAddr *net.TCPAddr
	RemoteAddr *net.TCPAddr
}

// 新建一个本地端
// 本地端的职责是:
// 1. 监听来自本机浏览器的代理请求
// 2. 转发前加密数据
// 3. 转发socket数据到墙外代理服务端
// 4. 把服务端返回的数据转发给用户的浏览器
func NewLsLocal(password string, listenAddr, remoteAddr string) (*LsLocal, error) {
	bsPassword, err := parsePassword(password) // 将明文密码转化为base64的密码
	if err != nil {
		return nil, err
	}
	structListenAddr, err := net.ResolveTCPAddr("tcp", listenAddr) // 监听本地连接数据
	if err != nil {
		return nil, err
	}
	structRemoteAddr, err := net.ResolveTCPAddr("tcp", remoteAddr) // 监听“翻墙”服务器数据
	if err != nil {
		return nil, err
	}
	return &LsLocal{
		Cipher:     newCipher(bsPassword),
		ListenAddr: structListenAddr,
		RemoteAddr: structRemoteAddr,
	}, nil
}

// 本地端启动监听，接收来自本机浏览器的连接
func (local *LsLocal) Listen(didListen func(listenAddr net.Addr)) error {
	return ListenSecureTCP(local.ListenAddr, local.Cipher, local.handleConn, didListen)
}

// 新的本机浏览器连接回调 本方法是协程
// 1 将网页的请求连接数据发送给代理服务器
// 2 同时创建一个协程将代理服务器的处理好的数据转发到本地的网页请求对象
func (local *LsLocal) handleConn(userConn *SecureTCPConn) {
	defer userConn.Close()

	proxyServer, err := DialTCPSecure(local.RemoteAddr, local.Cipher) // 连接代理服务器
	if err != nil {
		log.Println(err)
		return
	}
	defer proxyServer.Close()

	// Conn被关闭时直接清除所有数据 不管没有发送的数据
	//proxyServer.SetLinger(0)

	// 进行转发
	// 2 从 proxyServer 读取数据发送到 localUser
	go func() {
		err := proxyServer.DecodeCopy(userConn)
		if err != nil {
			// 在 copy 的过程中可能会存在网络超时等 error 被 return，只要有一个发生了错误就退出本次工作
			userConn.Close()
			proxyServer.Close()
		}
	}()
	// 1 从 localUser 发送数据发送到 proxyServer，这里因为处在翻墙阶段出现网络错误的概率更大
	userConn.EncodeCopy(proxyServer)
}
