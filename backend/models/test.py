from sqlmodel import Field, SQLModel


class Test(SQLModel, table=True):

    __tablename__ = "tests"

    id: int | None = Field(default=None, primary_key=True)
    name: str
    secret_name: str
    age: int | None = None
