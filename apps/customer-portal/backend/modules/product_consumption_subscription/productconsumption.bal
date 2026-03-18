// Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
//
// This software is the property of WSO2 LLC. and its suppliers, if any.
// Dissemination of any information or reproduction of any material contained
// herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
// You may not alter or remove any copyright or other notice from copies of this content.
# Creates an application for the project in Choreo.
#
# + return - Created application details or error
public isolated function test() returns json|error {

    return productConsumptionClient->/generate\-secret\-keys.post({});

}



# Processes the product consumption flow for a project based on its current status.
#
# + payload - Project status request containing email, deploymentId, and projectId
# + return - License details or completion message, or error
public isolated  function process(ProjectStatusRequest payload) returns json|error {

    // 1. Get current status
    Result statusRes =
        check productConsumptionClient->/projects/[payload.projectId]/consumption/status
            .post({
                email: payload.email,
                deploymentId: payload.deploymentId
            });


    int status = statusRes.result.status;
    string applicationId = statusRes.applicationId ?: "";

    // STATUS 1 → CREATE APPLICATION
    if status == 1 {
        ApplicationCreateResponse app =
            check productConsumptionClient->/applications
                .post({name: payload.projectId+"Test", description: "Application for project " + payload.projectId});

        applicationId = app.applicationId;

        Result _ = check productConsumptionClient->/projects/[payload.projectId]
            .patch({
                status: "2",
                applicationId: applicationId
            });

        status = 2;
    }

    // STATUS 2 → SUBSCRIBE
    if status == 2 {
        ApplicationSubscriptionResponse _ = check productConsumptionClient->/applications/[applicationId]/subscribe.post(<ApplicationSubscriptionPayload>{
            applicationId: applicationId
        });

        Result _ = check productConsumptionClient->/projects/[payload.projectId]
            .patch({
                status: "3"
            });

        status = 3;
    }

    // STATUS 3 → GENERATE CREDENTIALS
    if status == 3 {
        ApplicationKeyGenerationResponse creds =
            check productConsumptionClient->/applications/[applicationId]/generate\-credentials
                .post(<ApplicationKeyGenerationPayload>{
            keyType: "PRODUCTION",
            keyManager: "Resident Key Manager",
            grantTypesToBeSupported: ["client_credentials"]
        });

        Result _ = check productConsumptionClient->/projects/[payload.projectId]
            .patch({
                status: "4",
                consumerKey: creds.consumerKey,
                consumerSecret: creds.consumerSecret
            });

        status = 4;
    }

    // STATUS 4 → GENERATE SECRET KEYS
    if status == 4 {
        SecretKeysResponse keys =
            check productConsumptionClient->/generate\-secret\-keys.post({});

        Result _ = check productConsumptionClient->/projects/[payload.projectId]
            .patch({
                status: "5",
                primarySecretKey: keys.primarySecretKey,
                secondarySecretKey: keys.secondarySecretKey
            });

        status = 5;
    }

    // STATUS 5 → DOWNLOAD LICENSE
    if status == 5 {
        LicenseResponse license =
            check productConsumptionClient->/projects/[payload.projectId]/deployments/[payload.deploymentId]/license
                .post(
                    {
                        email: payload.email
                    }
                    );

        return license.toJson();
    }

    return {message: "Process completed"};
}
