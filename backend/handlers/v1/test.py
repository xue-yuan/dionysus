from fastapi import Depends
from sqlmodel import select, Session

from database import get_session
from models.test import Test


def get(session: Session = Depends(get_session)):
    try:
        with session.begin():
            test = Test.create(session)
            print(test)
    except Exception as e:
        print(e)
        ...

    statement = select(Test).where(Test.age <= 35)
    results = session.exec(statement)
    for hero in results:
        print(hero)

    return {"foo": "bar"}
