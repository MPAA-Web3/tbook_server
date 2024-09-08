import pymysql
import requests
import time
import asyncio
import logging
from TonTools import TonCenterClient
import redis  # 导入 Redis

# 配置日志记录
logging.basicConfig(
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    level=logging.INFO
)

logger = logging.getLogger(__name__)

# 预定义的 Ton 代币地址
ton_token_address = "0:C9179592448CE934A2FC8E11DF08EA0F2EFE8238A8DD4A40F11104826686F471"

# 数据库配置信息
db_config = {
    "user": "mpaa",
    "password": "Zz1234567",
    "host": "localhost",
    "port": 3306,
    "database": "tbooks"
}

# Redis 配置信息
redis_config = {
    "host": "localhost",
    "port": 6379,
    "password": "",
    "db": 0
}

# 积分比例常量
POINTS_CONVERSION_RATE = 5000  # 1积分对应5000单位

api_key = 'fb391c762f7e29326587d22f91b6747b9b93167b26dddb5ba37a89a08a894963'
client = TonCenterClient(api_key)

# 创建 Redis 连接
redis_client = redis.Redis(
    host=redis_config["host"],
    port=redis_config["port"],
    password=redis_config["password"],
    db=redis_config["db"]
)

async def get_transaction(address, boc_base64, max_attempts=60, delay=1):
    for _ in range(max_attempts):
        try:
            transactions = await client.get_transactions(address=address, limit=10)
            for tx in transactions:
                if hasattr(tx, 'hash'):
                    return tx.hash
        except Exception as e:
            print(f"Error fetching transactions: {e}")
        await asyncio.sleep(delay)
    raise TimeoutError("Transaction not found within the specified time")

async def process_orders():
    connection = pymysql.connect(user=db_config["user"],
                                 password=db_config["password"],
                                 host=db_config["host"],
                                 port=db_config["port"],
                                 database=db_config["database"])

    try:
        with connection.cursor() as cursor:
            # 查询 Status 为 'pending' 的订单
            select_query = """
            SELECT id, address, transaction_hash
            FROM `order`
            WHERE Status = 'pending'
            """
            cursor.execute(select_query)
            orders = cursor.fetchall()

            # 遍历每个订单并调用 API
            for order in orders:
                order_id, address, transaction_hash = order

                # 异步获取事务哈希
                try:
                    transaction_hash = await get_transaction(address, transaction_hash)
                except TimeoutError as e:
                    print(f"Order ID: {order_id} - {e}")
                    continue

                # API endpoint and parameters
                url = "https://toncenter.com/api/v3/transactions"
                params = {
                    "account": address,
                    "hash": transaction_hash,
                    "limit": 128,
                    "offset": 0,
                    "sort": "desc"
                }

                # 调用 API
                response = requests.get(url, params=params)

                if response.status_code == 200:
                    data = response.json()
                    # 处理和打印返回数据
                    transactions = data.get('transactions', [])
                    for tx in transactions:
                        account = tx.get('account')
                        tx_hash = tx.get('hash')
                        total_fees = tx.get('total_fees')
                        out_msgs = tx.get('out_msgs', [])  # 获取 out_msgs 列表

                        # 确保 total_fees 是数字
                        try:
                            total_fees = float(total_fees)
                        except ValueError:
                            print(f"Order ID: {order_id} - Total fees is not a valid number: {total_fees}")
                            continue

                        # 只更新状态当 bounce 为 False
                        destination = None
                        bounce_found = False
                        for msg in out_msgs:
                            bounce = msg.get('bounce')
                            destination = msg.get('destination')  # 获取消息的目标地址
                            if bounce is False:
                                bounce_found = True

                            print(f"Order ID: {order_id}")
                            print(f"  Account: {account}")
                            print(f"  Transaction Hash: {tx_hash}")
                            print(f"  Total Fees: {total_fees}")
                            print(f"  Bounce: {bounce}")  # 打印 bounce 字段
                            print("------------------------------")

                        # 核对是否是 Ton 代币地址
                        if destination == ton_token_address:
                            if bounce_found:
                                # 更新用户积分，比例为 1:5000
                                # 计算积分
                                points = total_fees / POINTS_CONVERSION_RATE

                                # 查询用户的 address
                                user_select_query = """
                                SELECT user_id FROM user WHERE address = %s
                                """
                                cursor.execute(user_select_query, (address,))
                                user = cursor.fetchone()

                                if user:
                                    user_id = user[0]

                                    # Redis 中的余额键
                                    balance_key = f"{user_id}_balance"

                                    # 更新 Redis 中的用户余额
                                    redis_client.incrbyfloat(balance_key, points)
                                    print(f"Updated user {user_id} balance by {points} points in Redis.")

                                # 更新订单 Status 为 'true'
                                update_query = """
                                UPDATE `order`
                                SET Status = 'true'
                                WHERE ID = %s
                                """
                                cursor.execute(update_query, (order_id,))
                                connection.commit()
                                break  # 只需检查第一个 bounce 为 False 的情况


                else:
                    print(f"Order ID: {order_id} - API 请求失败, 状态码: {response.status_code}")

    finally:
        # 关闭数据库连接
        connection.close()

async def main():
    while True:
        await process_orders()
        logger.info("机器人正在监听...")
        # 每10秒执行一次
        await asyncio.sleep(10)

if __name__ == "__main__":
    asyncio.run(main())