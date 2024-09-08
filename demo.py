import logging
from datetime import datetime
from sqlalchemy import create_engine, Column, String, Integer, DateTime, Float, Boolean
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker
from sqlalchemy.exc import OperationalError
from telegram import InlineQueryResultArticle, InputTextMessageContent, Update, Bot, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import Application, InlineQueryHandler, CommandHandler, CallbackContext
import asyncio
import redis

# 配置日志记录
logging.basicConfig(
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    level=logging.INFO
)

logger = logging.getLogger(__name__)

# Redis 配置信息
redis_config = {
    "host": "localhost",
    "port": 6379,
    "password": "",
    "db": 0
}

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
Base = declarative_base()
engine = create_engine(
    DATABASE_URL,
    pool_size=10,
    max_overflow=20,
    pool_timeout=30,
    pool_recycle=1800,  # 每隔30分钟回收连接
    pool_pre_ping=True  # 在每次使用连接前检查连接是否有效
)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

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

# 检查用户是否为会员
async def is_premium_user(bot: Bot, user_id: int) -> bool:
    try:
        user = await bot.get_chat_member(chat_id=user_id, user_id=user_id)
        return user.status == 'member' and user.user.is_premium
    except Exception as e:
        logger.error(f"Error checking premium status: {e}")
        return False

# 安全的 Telegram API 调用，带重试机制
async def safe_telegram_call(context: CallbackContext, update: Update, func, retries=3):
    for attempt in range(retries):
        try:
            await func()
            break  # 成功后跳出循环
        except Exception as e:
            logger.error(f"Attempt {attempt + 1} failed: {e}")
            if attempt + 1 == retries:
                logger.error("Max retries reached, giving up.")
            else:
                await asyncio.sleep(2)  # 等待一段时间后重试

async def start(update: Update, context: CallbackContext):
    session = SessionLocal()  # 创建数据库会话
    try:
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
            # Redis 中的余额键
            balance_key = f"{referral_id}_balance"

            # 检查用户是否是会员
            bot = context.bot
            is_premium = await is_premium_user(bot, int(user_id))
            if is_premium:
                # 会员用户+2500
                redis_client.incrbyfloat(balance_key, 2500)
                card_count = 15
            else:
                # 非会员用户+500
                redis_client.incrbyfloat(balance_key, 500)
                card_count = 3

            # 更新 Redis 中的卡片数量
            card_count_key = f"{referral_id}_card_count"
            redis_client.incrbyfloat(card_count_key, card_count)

            logger.info(f'Redis 中的用户 {referral_id} 余额更新为: {redis_client.get(balance_key)}')
            logger.info(f'Redis 中的用户 {referral_id} 卡片数量更新为: {redis_client.get(card_count_key)}')

        # 创建按钮
        buttons = [
            InlineKeyboardButton('🕹Play', url='https://t.me/rabbitluckbot/admin?startapp=pop4'),
        ]

        if buttons:
            keyboard = InlineKeyboardMarkup.from_column(buttons)
            await safe_telegram_call(context, update, lambda: context.bot.send_photo(
                chat_id=update.effective_chat.id,
                photo='https://blue-worthwhile-lobster-181.mypinata.cloud/ipfs/QmfYcxGzDxLWjXeLUP6GcN4wP8EZHxUnFQcSCmGacX1MS2',
                caption='🥳Grab Your Lucky!\n'
                '🙌Scratch tickets and spin the wheel to win mystery prizes.\n\n'
                '🍻 Click “Play”, unleash Your Luck with the Ultimate Web3 Scratch-Off Game!\n\n'
                '🐰 Play "Scratch and Win" to get your lucky cards and share with your friends to earn more. 🎉\n\n'
                '🐰 Try "Lucky Spin" to unlock the ultimate prize!',
                reply_markup=keyboard  # 附上键盘
            ))

        if referral_id:
            # 检查推荐用户是否存在
            invitee_user = session.query(User).filter(User.user_id == referral_id).first()
            if not invitee_user:
                await update.message.reply_text(f'推荐用户 ID: {referral_id} 不存在，请检查邀请 ID。')
                return

            # 检查被邀请用户是否是会员
            is_premium = await is_premium_user(context.bot, int(user_id))
            invitee_address = 'true' if is_premium else ''

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
                    invitee_address=invitee_address,  # 设置 invitee_address
                    level=1,
                )
                session.add(new_invitation)
                session.commit()
                logger.info(f'新邀请记录创建: {user_id} 邀请 {referral_id}')
    except OperationalError as e:
        logger.error(f"MySQL OperationalError: {e}")
        session.rollback()  # 回滚事务
    except Exception as e:
        logger.error(f"Error in start function: {e}")
    finally:
        session.close()  # 确保会话被关闭

async def explore_campaigns(update: Update, context: CallbackContext):
    buttons = [
        InlineKeyboardButton('🍻Explore Campaigns', url='https://t.me/rabbitluckbot/admin'),
    ]

    if buttons:
        keyboard = InlineKeyboardMarkup.from_column(buttons)
        await safe_telegram_call(context, update, lambda: update.message.reply_text(
            '🚀 Engage the best airdrops by tap on Explore Campaigns.', reply_markup=keyboard
        ))

async def support(update: Update, context: CallbackContext):
    message = (
        'Dear community, please feel free to contact us anytime while participating, and earning with Rabbit Luck.\n\n'
        '🏅Lucky Points\n'
        'Showcase your contributions and impact. Flagship TON Projects will see you!\n\n'
        '🚀Incentive Hub\n'
        'Explore and engage in the most potential projects and earn rewards!\n\n'
        '📖QA Doc\n'
        'If you have any other questions, please first look for answers in the QA section.\n'
        'Link: https://medium.com/@RabbitLuck\n\n'
        '📪Report\n'
        'For any inquiries or feedback of Rabbit Luck products, please don’t hesitate to reach out to us via Telegram: https://t.me/Rabbitlucksupportbot.'
    )

    await update.message.reply_text(message)

def main():
    application = Application.builder().token(TOKEN).build()

    application.add_handler(CommandHandler("start", start))
    application.add_handler(CommandHandler("support", support))
    application.add_handler(CommandHandler("explore_campaigns", explore_campaigns))

    application.run_polling()

if __name__ == '__main__':
    main()