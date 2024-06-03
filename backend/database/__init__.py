from sqlalchemy.engine import URL
from sqlmodel import create_engine, Session, SQLModel

import config
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


def get_tx():
    with Session(engine) as session:
        yield session


def initialize():
    SQLModel.metadata.create_all(bind=engine)
