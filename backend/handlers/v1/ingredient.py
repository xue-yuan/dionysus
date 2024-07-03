from fastapi import Depends
from sqlalchemy.exc import NoResultFound
from sqlmodel import Session

from constants.error import ClientError
from database import get_session
from handlers.v1.models import ErrorResponse
from handlers.v1.models.ingredient import CreateRequest, UpdateRequest, Response
from models.recipe import Ingredient
from utils.exceptions import BadRequestException
from utils.responses import EmptyResponse, ObjectResponse


def get(id: int, session: Session = Depends(get_session)) -> Response:
    try:
        return Ingredient.get_by_id(session, id)
    except NoResultFound as e:
        raise BadRequestException(error_code=ClientError.RESULT_NOT_FOUND)


def create(body: CreateRequest, session: Session = Depends(get_session)) -> Response:
    with session.begin():
        return ObjectResponse(
            Ingredient.create(session, body.name, body.unit, body.type)
        )


def update(
    id: int, body: UpdateRequest, session: Session = Depends(get_session)
) -> Response:
    with session.begin():
        Ingredient.update(session, id, body.name)
        return Ingredient.get_by_id(session, id)


def delete(id: int, session: Session = Depends(get_session)):
    with session.begin():
        Ingredient.delete(session, id)
        return EmptyResponse()
