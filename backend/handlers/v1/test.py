from fastapi import Depends
from sqlmodel import Session, select

from database import get_tx
from models.test import Test


def get(*, tx: Session = Depends(get_tx)):
    statement = select(Test).where(Test.age <= 35)
    results = tx.exec(statement)
    for hero in results:
        print(hero)

    return {"foo": "bar"}
