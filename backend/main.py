from contextlib import asynccontextmanager

from fastapi import FastAPI

from routers import apiRouter
from utils import get_openapi


@asynccontextmanager
async def lifespan(app: FastAPI):
    # setup

    yield
    # teardown

app = FastAPI(lifespan=lifespan)
app.openapi = get_openapi(app)
app.include_router(apiRouter)
