from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.exceptions import HTTPException
from sqlalchemy.exc import IntegrityError

from routers import apiRouter
from utils import get_openapi
from utils.exceptions import (
    CustomException, custom_exception_handler,
    global_exception_handler, http_exception_handler,
    integrity_exception_handler
)


@asynccontextmanager
async def lifespan(app: FastAPI):
    # setup

    yield
    # teardown

app = FastAPI(lifespan=lifespan)
app.openapi = get_openapi(app)
app.include_router(apiRouter)
app.add_exception_handler(Exception, global_exception_handler)
app.add_exception_handler(CustomException, custom_exception_handler)
app.add_exception_handler(HTTPException, http_exception_handler)
app.add_exception_handler(IntegrityError, integrity_exception_handler)
