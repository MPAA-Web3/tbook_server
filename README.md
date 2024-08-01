# tbook_server

export GOOS=darwin                                    
export GOARCH=amd64

GO111MODULE="on" CGO_ENABLED=0 GOOS=linux GOARCH=amd64
go build -o main main.go

ps aux | grep python

nohup python demo.py > demo.log 2>&1 &

sudo apt install python3-venv
python3 -m venv myenv
source myenv/bin/activate
pip install sqlalchemy python-telegram-bot pymysql
nohup python demo.py > demo.log 2>&1 &