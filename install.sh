sudo mkdir /opt/terarrium

go get

go mod tidy

go build -o terarrium-app .

sudo mv terarrium-app /opt/terarrium-app

sudo cp ./terarrium.service /etc/systemd/system/terarrium.service
11
sudo systemctl daemon-reload
sudo systemctl enable terrarium.service
sudo systemctl start terrarium.service