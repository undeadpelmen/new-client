sudo mkdir /opt/terarrium

go build -o ./cmd/server/main.go terarrium

mv terarrium /opt/terarrium

cp ./terarrium.service /etc/systemd/system/terarrium.service