# go-uploader

一个基于Go语言的文件分片上传服务器。

## 功能特点

- 支持大文件分片上传
- 支持断点续传
- 支持MD5校验
- 支持自定义上传和存储目录
- 支持自定义服务端口

## 配置说明

程序启动时会自动加载 `config.json` 配置文件，如果配置文件不存在，将使用默认配置并创建配置文件。

配置项说明：

```json
{
  "upload_dir": "./upload",  // 上传临时文件存储目录
  "merged_dir": "./merged",  // 合并后文件存储目录
  "port": "18101"            // 服务器监听端口
}
```

这些目录会在程序启动时自动检查，如果不存在则会自动创建。

## 启动方式

```bash
# 直接运行
go run main.go

# 或者使用脚本启动
./start.sh
```

## API接口

- `/go-uploader/upload_chunk` - 上传文件分片
- `/go-uploader/merge_chunks` - 合并文件分片
- `/go-uploader/upload_status` - 查询上传状态