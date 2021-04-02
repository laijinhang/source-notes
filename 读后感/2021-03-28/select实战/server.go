package main

/*
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <errno.h>
#include <string.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <sys/time.h>
#include <netinet/in.h>
#include <arpa/inet.h>

#define MYPORT 10001 	// 连接时使用的端口
#define MAXCLINE 10		// 连接队列中的个数
#define BUF_SIZE 1024	// 缓存大小

int fd[MAXCLINE];	// 连接的fd
int conn_amount;	// 当前的连接数

void showclient()
{
    printf("client amount:%d\n",conn_amount);
    for(int i = 0;i < MAXCLINE;i++)
    {
        printf("[%d]:%d ", i, fd[i]);
    }
    printf("\n\n");
}
int server()
{
    int sock_fd, new_fd; 			// 监听套接字 连接套接字
    struct sockaddr_in server_addr; // 服务器的地址信息
    struct sockaddr_in client_addr; // 客户端的地址信息
    socklen_t sin_size;
    int yes = 1;
    char buf[BUF_SIZE];
    int ret;
    int i;
    if((sock_fd = socket(AF_INET,SOCK_STREAM,0))==-1)
    {
        perror("setsockopt");
        exit(1);
    }
    printf("sockect_fd = %d\n", sock_fd);
    if(setsockopt(sock_fd,SOL_SOCKET,SO_REUSEADDR,&yes,sizeof(int))==-1)
    {
        perror("setsockopt error \n");
        exit(1);
    }

    server_addr.sin_family = AF_INET;
    server_addr.sin_port = htons(MYPORT);
    server_addr.sin_addr.s_addr = INADDR_ANY;
    memset(server_addr.sin_zero, '\0', sizeof(server_addr.sin_zero));
    if(bind(sock_fd, (struct sockaddr *)&server_addr, sizeof(server_addr)) == -1)
    {
        perror("bind error!\n");
        exit(1);
    }
    if(listen(sock_fd, MAXCLINE)==-1)
    {
        perror("listen error!\n");
        exit(1);
    }
    printf("listen port %d\n",MYPORT);
    fd_set fdsr; // 文件描述符集的定义
    int maxsock;
    struct timeval tv;
    conn_amount =0;
    sin_size = sizeof(client_addr);
    maxsock = sock_fd;
    while(1)
    {
        FD_ZERO(&fdsr);			// 清空集合
        FD_SET(sock_fd, &fdsr);	// 将套接字加入到集合中

        // 设置超时时间，也就是说到了这个设置的时间，一定会结束等待
		// 如果没有设置超时时间，那么会进入阻塞状态
		tv.tv_sec = 30;
        tv.tv_usec =0;

        // 将所有的连接全部加到这个这个集合中，可以监测客户端是否有数据到来
        for(i = 0; i < MAXCLINE; i++)
        {
            if(fd[i] != 0)
            {
                FD_SET(fd[i],&fdsr);
            }
        }
        // 如果文件描述符中有连接请求 会做相应的处理，实现I/O的复用 多用户的连接通讯
        ret = select(maxsock +1, &fdsr, NULL, NULL, &tv);
        if(ret < 0) // 没有找到有效的连接 失败
        {
            perror("select error!\n");
            break;
        }
        else if(ret == 0)// 指定的时间到，
        {
            printf("timeout \n");
            continue;
        }
        for(i = 0;i < conn_amount;i++)
        {
			// 检查集合中指定的文件描述符是否可以读写
            if(FD_ISSET(fd[i], &fdsr))
            {
                ret = recv(fd[i], buf, sizeof(buf), 0);
                if(ret <= 0)
                {
					printf("client[%d] close\n",i);
					close(fd[i]);
					// 将这个已经关闭的连接套接字从这个集合中删除
					FD_CLR(fd[i],&fdsr);
					fd[i]=0;
					conn_amount--;
                }
                else
                {
                    if(ret < BUF_SIZE)
                        memset(&buf[ret],'\0',1);
                    printf("client[%d] send >> %s\n",i, buf);
                }
            }
        }
		// 处理新的连接
        if(FD_ISSET(sock_fd,&fdsr))
        {
            new_fd = accept(sock_fd,(struct sockaddr *)&client_addr,&sin_size);
            if(new_fd <=0)
            {
                perror("accept error\n");
                continue;
            }
            if(conn_amount < MAXCLINE)
            {
                for(i = 0; i < MAXCLINE; i++)
                {
                    if(fd[i]==0)
                    {
                        fd[i] = new_fd;
                        break;
                    }
                }
                conn_amount++;
                printf("new connection client[%d]%s:%d\n",conn_amount,inet_ntoa(client_addr.sin_addr),ntohs(client_addr.sin_port));
                if(new_fd > maxsock)
                {
                    maxsock = new_fd;
                }
            }
            else
            {
                printf("max connections arrive ,exit\n");
                send(new_fd,"bye",4,0);
                close(new_fd);
                continue;
            }
        }
        showclient();
    }

    for(i=0;i<MAXCLINE;i++)
    {
        if(fd[i]!=0)
        {
            close(fd[i]);
        }
    }
	exit(0);
}
*/
import "C"

func main() {
	C.server()
}
