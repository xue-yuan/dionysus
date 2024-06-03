from sqlalchemy.orm import Session
from sqlmodel import Field, SQLModel


class Test(SQLModel, table=True):

    __tablename__ = "tests"

    id: int | None = Field(default=None, primary_key=True)
    name: str
    secret_name: str
    age: int | None = None

    @classmethod
    def create(cls, s: Session, name="test", secret_name="test", age=12):
        test = cls(name=name, secret_name=secret_name, age=age)

        s.add(test)
        return test
