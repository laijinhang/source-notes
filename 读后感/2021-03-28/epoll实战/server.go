package main

/*
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>

#include <netinet/in.h>
#include <sys/socket.h>
#include <arpa/inet.h>
#include <sys/epoll.h>
#include <unistd.h>
#include <sys/types.h>

const int MAX_EPOLL_EVENTS = 1000;
const int MAX_MSG_LEN = 1024;

void err_exit(const char *s){
    printf("error: %s\n",s);
    exit(0);
}

int create_socket(char* ip, int port_number)
{
    struct sockaddr_in server_addr = {0};
	server_addr.sin_family = AF_INET;
	server_addr.sin_port = htons(port_number);
	if(inet_pton(server_addr.sin_family, ip, &server_addr.sin_addr) == -1){
		err_exit("inet_pton");
	}
	// 1、创建套接字
	int sockfd = socket(PF_INET, SOCK_STREAM, 0);
	if(sockfd == -1){
		err_exit("socket");
	}
	int reuse = 1;
	if(setsockopt(sockfd, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse)) == -1)
	{
		err_exit("setsockopt");
	}
	// 2、绑定监听端口
	if(bind(sockfd, (struct sockaddr *)&server_addr, sizeof(server_addr)) == -1){
		err_exit("bind");
	}
	// 3、监听
	if(listen(sockfd, 5) == -1){
		err_exit("listen");
	}
	return sockfd;
}

struct epoll_event event;   // 告诉内核要监听什么事件
struct epoll_event wait_event, events[30]; //内核监听完的结果

int set_epoll_event_fd(int socket_fd) {
	epoll_data_t d;
	d.fd = socket_fd;
	event.data = d;
}

int c_epoll_wait(int epfd) {
	return epoll_wait(epfd, events, 30, 500);
}

#define    MAXLINE        4096

int handle(int socket_fd, int epfd) {
	int nfds, connfd;
    socklen_t clilen;
    ssize_t n;
    char BUF[MAXLINE];
    struct sockaddr_in cliaddr, servaddr;
	for(;;) {
		nfds = epoll_wait(epfd, events, 30, 500);
		for(int i = 0;i < nfds;i++) {
			printf("nfds: %d, socket_fd: %d,events[%d].data.fd: %d,connfd: %d\n", nfds, socket_fd, i, events[i].data.fd,connfd);
			if(events[i].data.fd == socket_fd) {
				connfd = accept(socket_fd, (struct sockaddr *)&cliaddr, &clilen);
				if(connfd < 0) {
					perror("connfd<0");
					exit(1);
				}
                char *str = inet_ntoa(cliaddr.sin_addr);
                printf("accapt a connection from %s\n", str);

				struct epoll_event ev;
                ev.data.fd = connfd;
                ev.events=EPOLLIN|EPOLLET;
                epoll_ctl(epfd, EPOLL_CTL_ADD, connfd, &ev);
			} else if(events[i].events&EPOLLIN) { //如果是已经连接的用户，并且收到数据，那么进行读入。
                if ( (connfd = events[i].data.fd) < 0)
                    continue;
                if ( (n = read(connfd, BUF, MAXLINE)) < 0) {
                    if (errno == ECONNRESET) {
                        close(connfd);
                        events[i].data.fd = -1;
                    } else {
                        printf("readline error\n");
					}
                } else if (n == 0) {
					printf("err\n");
                    close(connfd);
                    events[i].data.fd = -1;
                }
                BUF[n] = '\0';
				printf("%s %d\n", BUF, n);
                event.data.fd=connfd;
                event.events=EPOLLIN|EPOLLET;
				//读完后准备写
                epoll_ctl(epfd, EPOLL_CTL_MOD, connfd, &event);
			} else if(events[i].events&EPOLLOUT) // 如果有数据发送
            {
                //socket_fd = events[i].data.fd;
                //write(socket_fd, BUF, n);
				//
                //event.data.fd = socket_fd;
                //event.events = EPOLLIN|EPOLLET;
                //写完后，这个sockfd准备读
                //epoll_ctl(epfd, EPOLL_CTL_MOD, socket_fd, &event);
            }
		}
	}
}
*/
import "C"

func CreateSocker(ip string, port int) int {
	return int(C.create_socket(C.CString(ip), C.int(port)))
}

func main() {
	socketFd := CreateSocker("127.0.0.1", 10001)
	if socketFd < 0 {
		panic("socket err")
	}
	epfd := C.epoll_create(10)
	if epfd < 0 {
		panic("epoll_create")
	}
	C.set_epoll_event_fd(C.int(socketFd))
	C.event.events = C.EPOLLIN
	// 事件注册函数，将监听套接字描述符 sockfd 加入监听事件
	ret := C.epoll_ctl(epfd, C.EPOLL_CTL_ADD, C.int(socketFd), &C.event)
	if ret < 0 {
		panic("epoll_ctl")
	}
	C.handle(C.int(socketFd), epfd)
}
