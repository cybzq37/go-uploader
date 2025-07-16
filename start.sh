docker run -d --name uploader \
  -p 18101:18101 \
  -v $(pwd)/upload:/app/upload \
  -v $(pwd)/merged:/app/merged \
  go-uploader
