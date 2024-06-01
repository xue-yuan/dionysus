from handlers import heartbeat
from utils import APIRouter


apiRouter = APIRouter(prefix="/api")
v1Router = APIRouter(prefix="/v1")
userRouter = APIRouter(prefix="/user", tags=["user"])

userRouter.add_api_route("", heartbeat)
userRouter.add_api_auth_route("/test_user", heartbeat)

apiRouter.add_api_route("/heartbeat", heartbeat)

v1Router.include_router(userRouter)
apiRouter.include_router(v1Router)
