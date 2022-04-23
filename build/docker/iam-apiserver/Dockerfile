# Copyright 2020 Lingfei Kong <colin404@foxmail.com>. All rights reserved.    
# Use of this source code is governed by a MIT style    
# license that can be found in the LICENSE file.

# 定义基础镜像的指令：FROM；
FROM BASE_IMAGE
# 定义镜像维护者的信息
MAINTAINER Lingfei Kong <colin404@foxmail.com>

# 镜像的操作指令
RUN ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime     
RUN echo "Asia/Shanghai" > /etc/timezone 

WORKDIR /opt/iam
COPY iam-apiserver /opt/iam/bin/

# 容器启动时执行指令
ENTRYPOINT ["/opt/iam/bin/iam-apiserver"]
