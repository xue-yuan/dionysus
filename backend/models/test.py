from sqlalchemy import INTEGER, Column

from models.base import Base


class Test(Base):
    __tablename__ = "test"

    id = Column(INTEGER, primary_key=True, index=True)
