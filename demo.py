import logging
import uuid
from datetime import datetime
from sqlalchemy import create_engine, Column, String, Integer, DateTime, Float, Boolean
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker
from telegram import InlineQueryResultArticle, InputTextMessageContent, Update, Bot
from telegram.ext import Application, InlineQueryHandler, CommandHandler, CallbackContext
from telegram import InlineKeyboardButton, InlineKeyboardMarkup
import asyncio
import redis  # 导入 Redis

# 配置日志记录
logging.basicConfig(
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    level=logging.INFO
)

# Redis 配置信息
redis_config = {
    "host": "localhost",
    "port": 6379,
    "password": "",
    "db": 0
}

logger = logging.getLogger(__name__)
# 创建 Redis 连接
redis_client = redis.Redis(
    host=redis_config["host"],
    port=redis_config["port"],
    password=redis_config["password"],
    db=redis_config["db"]
)

# 你的机器人令牌
TOKEN = '7213972053:AAGb5UGTR0Lk8DExbv6epsUu2SY7ciWuR5c'
# 数据库配置
DATABASE_URL = "mysql+pymysql://mpaa:Zz1234567@localhost:3306/tbooks"

# SQLAlchemy 初始化
                                                                                                           1,14          Top