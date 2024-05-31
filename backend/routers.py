from utils import APIRouter


def heartbeat():
    return {"foo": "bar"}


def get_user():
    return {"foo": "bar"}


apiRouter = APIRouter(prefix="/api")
v1Router = APIRouter(prefix="/v1")
userRouter = APIRouter(prefix="/user", tags=["user"])

v1Router.add_api_route("/user", get_user)
v1Router.add_api_auth_route("/test_user", get_user)

apiRouter.add_api_route("/heartbeat", heartbeat, tags=["test"])
apiRouter.include_router(v1Router)
