# 一体化镜像：前端构建 → 后端构建（内嵌 dist）→ 单运行时容器
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm config set registry https://registry.npmmirror.com && npm install
COPY frontend/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend
WORKDIR 
ENV GOPROXY=https://goproxy.cn,direct
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/fenx-server ./cmd/server

FROM alpine:3.20
RUN adduser -D -H fenx
USER fenx
COPY --from=backend /out/fenx-server /fenx-server
COPY --from=frontend /app/dist /public
ENV FRONTEND_DIST=/public
EXPOSE 8080
ENTRYPOINT ["/fenx-server"]
