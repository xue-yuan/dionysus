from sqlalchemy import create_engine
from sqlalchemy.engine import URL
from sqlalchemy.orm import sessionmaker

import config

from models.base import Base
from models.test import Test


url = URL.create(
    drivername=config.DATABASE_DRIVERNAME,
    database=config.DATABASE_NAME,
    username=config.DATABASE_USERNAME,
    password=config.DATABASE_PASSWORD,
    host=config.DATABASE_HOST,
    port=config.DATABASE_PORT,
)


engine = create_engine(
    url,
    echo=True,
    max_overflow=config.DATABASE_MAX_OVERFLOW,
    pool_recycle=config.DATABASE_POOL_RECYCLE,
    pool_size=config.DATABASE_POOL_SIZE,
    pool_timeout=config.DATABASE_POOL_TIMEOUT,
)

Session = sessionmaker(bind=engine, expire_on_commit=False)


def get_tx():
    return Session


def initialize():
    Base.metadata.create_all(bind=engine)
