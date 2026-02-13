sudo mkdir /opt/terarrium

go mod init github.com/undeadpelmen/new-client

go mor tidy

go get

go build -o ./cmd/server/main.go terarrium

sudo mv terarrium /opt/terarrium

sydo cp ./terarrium.service /etc/systemd/system/terarrium.service

sudo systemctl daemon-reload
sudo systemctl enable terrarium.service
sudo systemctl start terrarium.service