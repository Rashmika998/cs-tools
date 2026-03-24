// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
import ballerina/http;
import ballerina/jwt;
import ballerina/log;

public configurable AppRoles authorizedRoles = ?;
configurable TokenValidatorConfig tokenValidatorConfig = ?;

final jwt:ValidatorConfig & readonly jwtConfig = {
    issuer: tokenValidatorConfig.issuer,
    audience: tokenValidatorConfig.audience,
    clockSkew: tokenValidatorConfig.clockSkew,
    signatureConfig: {
        jwksConfig: {url: tokenValidatorConfig.jwksEndPoint}
    }
};

# Extracts and validates user info from JWT headers in an HTTP request.
# This function is used by both the HTTP interceptor and the WebSocket upgrade resource.
#
# + req - The HTTP request containing JWT headers
# + return - UserInfoPayload on success or error on validation failure
public isolated function getUserInfoFromRequest(http:Request req) returns UserInfoPayload|error {
    string|error idToken = req.getHeader(JWT_ASSERTION_HEADER);
    if idToken is error {
        string errorMsg = "Missing invoker info header!";
        log:printError(errorMsg, idToken);
        return error(errorMsg);
    }

    // TODO: Remove this if the token issuer issue get resolved.
    string|error userIdToken = req.getHeader(USER_ID_TOKEN_HEADER);
    if userIdToken is error {
        string errorMsg = "Missing user id token info header!";
        log:printError(errorMsg, userIdToken);
        return error(errorMsg);
    }

    // Validate JWT token
    jwt:Payload|error payload = jwt:validate(idToken, jwtConfig.cloneReadOnly());
    if payload is error {
        string errorMsg = "Invalid or expired token!";
        log:printError(errorMsg, payload);
        return error(errorMsg);
    }

    CustomJwtPayload|error payloadData = payload.cloneWithType(CustomJwtPayload);
    if payloadData is error {
        string errorMsg = "Malformed JWT payload!";
        log:printError(errorMsg, payloadData);
        return error(errorMsg);
    }

    return {
        email: payloadData.email,
        groups: payloadData.groups,
        userId: payloadData.userid,
        idToken: userIdToken
    };
}

# Validates user info from raw token strings.
# Used when tokens are received outside of standard HTTP headers (e.g., via Sec-WebSocket-Protocol).
#
# + jwtAssertion - The JWT assertion token (x-jwt-assertion equivalent)
# + userIdToken - The user ID token (x-user-id-token equivalent)
# + return - UserInfoPayload on success or error on validation failure
public isolated function getUserInfoFromTokens(string jwtAssertion, string userIdToken) returns UserInfoPayload|error {
    jwt:Payload|error payload = jwt:validate(jwtAssertion, jwtConfig.cloneReadOnly());
    if payload is error {
        string errorMsg = "Invalid or expired token!";
        log:printError(errorMsg, payload);
        return error(errorMsg);
    }

    CustomJwtPayload|error payloadData = payload.cloneWithType(CustomJwtPayload);
    if payloadData is error {
        string errorMsg = "Malformed JWT payload!";
        log:printError(errorMsg, payloadData);
        return error(errorMsg);
    }

    return {
        email: payloadData.email,
        groups: payloadData.groups,
        userId: payloadData.userid,
        idToken: userIdToken
    };
}

# To handle authorization for each resource function invocation.
public isolated service class JwtInterceptor {

    *http:RequestInterceptor;

    isolated resource function default [string... path](http:RequestContext ctx, http:Request req)
        returns http:NextService|http:Unauthorized|http:InternalServerError|error? {

        UserInfoPayload|error userInfo = getUserInfoFromRequest(req);
        if userInfo is error {
            return <http:InternalServerError>{body: {message: userInfo.message()}};
        }

        ctx.set(HEADER_USER_INFO, userInfo);
        return ctx.next();
    }
}
