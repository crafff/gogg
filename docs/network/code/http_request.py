import socket

def send_http_request():
    # 1. 定义目标服务器和路径
    # 我们访问一个专门用来测试 HTTP 请求的公开网站：httpbin.org
    host = "httpbin.org"
    port = 80
    path = "/get?name=Ruitao&type=test"

    # 2. 手动拼接标准的 HTTP 请求报文 (注意每个 SP 空格和末尾的 \r\n)
    # 结构：METHOD SP URI SP HTTP-Version CRLF
    request_line = f"GET {path} HTTP/1.1\r\n"
    headers = f"Host: {host}\r\n" + \
              "User-Agent: MyCustomPythonClient/1.0\r\n" + \
              "Accept: */*\r\n" + \
              "Connection: close\r\n"  # 告诉服务器完事儿就断开，方便我们判断结束

    # 请求头和请求体之间必须有一个空行（再加一个 \r\n）
    full_request = request_line + headers + "\r\n"

    print("=" * 20 + " 发出的 原始 Request " + "=" * 20)
    # 把 \r\n 替换成可见的换行打印出来
    print(full_request)
    print("=" * 55 + "\n")

    # 3. 创建 TCP Socket 并连接服务器
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.connect((host, port))

    # 4. 发送请求 (需要将字符串编码为字节流)
    s.sendall(full_request.encode('utf-8'))

    # 5. 接收服务器的响应
    response_bytes = b""
    while True:
        chunk = s.recv(4096)
        if not chunk:
            break
        response_bytes += chunk

    s.close()

    # 6. 打印收到的 原始 Response
    print("=" * 20 + " 收到的 原始 Response " + "=" * 20)
    print(response_bytes.decode('utf-8'))
    print("=" * 56)

if __name__ == "__main__":
    send_http_request()
