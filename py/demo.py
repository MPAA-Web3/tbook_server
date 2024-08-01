import logging
import uuid
from datetime import datetime
from sqlalchemy import create_engine, Column, String, Integer, DateTime, Float, Boolean
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker
from telegram import InlineQueryResultArticle, InputTextMessageContent, Update
from telegram.ext import Application, InlineQueryHandler, CommandHandler, CallbackContext

# 配置日志记录
logging.basicConfig(
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    level=logging.INFO
)

logger = logging.getLogger(__name__)
# 你的机器人令牌
TOKEN = '7490052235:AAEnmFyYk-yoWraYisC-KC2UwhvAl6Z716E'
# 数据库配置
DATABASE_URL = "mysql+pymysql://mpaa:Zz1234567@rm-bp1tq0i62719ipf9tmo.mysql.rds.aliyuncs.com:3306/tbooks"

# SQLAlchemy 初始化
Base = declarative_base()
engine = create_engine(DATABASE_URL)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
session = SessionLocal()

# 用户模型
class User(Base):
    __tablename__ = 'user'

    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(String, unique=True, index=True, nullable=False)
    address = Column(String, index=True)
    created_at = Column(DateTime, default=datetime.utcnow)
    updated_at = Column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
    balance = Column(Float, default=0.0)
    card_count = Column(Integer, default=0)
    profile_photo = Column(String)
    joined_discord = Column(Boolean, default=False)
    joined_x = Column(Boolean, default=False)
    joined_telegram = Column(Boolean, default=False)

# 邀请模型
class Invitation(Base):
    __tablename__ = 'invitation'

    id = Column(Integer, primary_key=True, index=True)
    inviter_id = Column(String, nullable=False)
    inviter_address = Column(String, nullable=False)
    invitee_user_id = Column(String, nullable=False)
    invitee_address = Column(String, nullable=False)
    level = Column(Integer, nullable=False)
    created_at = Column(DateTime, default=datetime.utcnow)

async def start(update: Update, context: CallbackContext):
    args = context.args
    user_id = str(update.effective_user.id)
    referral_id = args[0] if args else None

    # 检查用户是否存在，如果不存在则创建用户
    existing_user = session.query(User).filter(User.user_id == user_id).first()
    if not existing_user:
        new_user = User(user_id=user_id, address='', balance=0.0, card_count=0)
        session.add(new_user)
        session.commit()
        logger.info(f'新用户创建: {user_id}')

    if referral_id:
        # 检查用户是否存在
        invitee_user = session.query(User).filter(User.user_id == referral_id).first()
        if not invitee_user:
            await update.message.reply_text(f'推荐用户 ID: {referral_id} 不存在，请检查邀请 ID。')
            return

        # 记录邀请关系，避免重复
        existing_invitation = session.query(Invitation).filter(
            Invitation.inviter_id == referral_id,
            Invitation.invitee_user_id == user_id
        ).first()
        if not existing_invitation:
            new_invitation = Invitation(
                inviter_id=referral_id,
                inviter_address='',  # 需要实际的地址
                invitee_user_id=user_id,
                invitee_address='',  # 需要实际的地址
                level=1,
            )
            session.add(new_invitation)
            session.commit()
            logger.info(f'新邀请记录创建: {user_id} 邀请 {referral_id}')

    await update.message.reply_text(f'https://t.me/testrabbitluckbot/admin')

def main():
    application = Application.builder().token(TOKEN).build()

    application.add_handler(CommandHandler('start', start))
  
    logger.info("机器人正在监听...")

    application.run_polling()

if __name__ == '__main__':
    main()
