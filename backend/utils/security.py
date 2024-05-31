from fastapi import HTTPException
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from fastapi.security.utils import get_authorization_scheme_param
from starlette.requests import Request

from utils import token


# To verify whether it is a valid jwt token
class JWTBearer(HTTPBearer):
    async def __call__(self, request: Request):
        authorization = request.headers.get("Authorization")
        scheme, credentials = get_authorization_scheme_param(authorization)
        if not (authorization and scheme and credentials):
            if self.auto_error:
                raise HTTPException(status_code=401)
            else:
                return None

        if scheme.lower() != "bearer":
            if self.auto_error:
                raise HTTPException(status_code=401)
            else:
                return None

        claim = token.validate(credentials)
        if not claim or token.is_in_blacklist(credentials):
            raise HTTPException(status_code=403)

        request.state.user_id = claim.get("user_id")

        return HTTPAuthorizationCredentials(scheme=scheme, credentials=credentials)


jwt_scheme = JWTBearer()
