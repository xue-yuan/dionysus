import os
from datetime import timedelta
from dotenv import load_dotenv


load_dotenv()

BASE_URL = os.getenv("BASE_URL")

SECRET_KEY = os.getenv("SECRET_KEY")

DATABASE_DRIVERNAME = "postgresql"
DATABASE_NAME = os.getenv("DATABASE_NAME")
DATABASE_USERNAME = os.getenv("DATABASE_USERNAME")
DATABASE_PASSWORD = os.getenv("DATABASE_PASSWORD")
DATABASE_HOST = os.getenv("DATABASE_HOST")
DATABASE_PORT = int(os.getenv("DATABASE_PORT"))

DATABASE_MAX_OVERFLOW = 0
DATABASE_POOL_RECYCLE = 1200
DATABASE_POOL_SIZE = 10
DATABASE_POOL_TIMEOUT = 20

REDIS_HOST = os.getenv("REDIS_HOST")
REDIS_PORT = int(os.getenv("REDIS_PORT"))
REDIS_PASSWORD = os.getenv("REDIS_PASSWORD")

TOKEN_TTL = timedelta(hours=2)  # 2 hours
OLD_TOKEN_TTL = timedelta(hours=1)  # 1 hour
