from fastapi.encoders import jsonable_encoder
from fastapi.responses import JSONResponse, Response
from starlette.status import HTTP_204_NO_CONTENT, HTTP_200_OK


class EmptyResponse(Response):
    def __init__(self):
        super().__init__(status_code=HTTP_204_NO_CONTENT)


class ObjectResponse(JSONResponse):
    media_type = "application/json"

    def __init__(
        self,
        content,
        status_code=HTTP_200_OK,
        headers=None,
        media_type=None,
        background=None,
    ):
        super().__init__(
            jsonable_encoder(content), status_code, headers, media_type, background
        )
