from fastapi import Depends
from sqlalchemy.exc import NoResultFound
from sqlmodel import Session

from constants.error import ClientError
from database import get_session
from models.recipe import Cocktail
from utils.exceptions import BadRequestException

from redis_helper import Redis


def get(id: int, session: Session = Depends(get_session)):
    try:
        Redis().set("TEST", 5, 100000)
        return {}
        # return Cocktail.get_by_id(session, id)
    except NoResultFound as e:
        raise BadRequestException(error_code=ClientError.RESULT_NOT_FOUND)


def get_page(session: Session = Depends(get_session)): ...


def create(session: Session = Depends(get_session)): ...


def update(session: Session = Depends(get_session)): ...


def delete(session: Session = Depends(get_session)): ...
