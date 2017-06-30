// What Docker Registry should we use for this project.
def dockerRegistry = "panamax.spectrumxg.com/star-dev/external-dns"
// Should we do a deployment.
// We don't do deployments for intermediary Docker images.
def shouldDeploy = false
// Allow a TAG AS LATEST Stage
def allowTagLatest = true
stage "Checkout Source"
node {
    checkout(scm)
}
// Get URL to SCM Repository (GIT)
def gUrl = scm.userRemoteConfigs[0].url.toURL()
def gUrlClean = gUrl.getProtocol() + "://__GITAUTH__@" + gUrl.getAuthority() + gUrl.getFile()
// Make docker image label
def dockerImageLabel = ("${BRANCH_NAME}" + "-" + "${BUILD_NUMBER}").replaceAll('[.*/]', '-').toLowerCase()
def dockerImageLatestLabel = ("${BRANCH_NAME}" + "-" + "latest").replaceAll('[.*/]', '-').toLowerCase()
node {
    stage "Build Docker"
    env.DKR_IMAGE_NAME = dockerRegistry
    env.DKR_IMAGE_LABEL = dockerImageLabel
    env.DKR_IMAGE_LATEST_LABEL = dockerImageLatestLabel
    withCredentials([usernamePassword(credentialsId: 'GitLabJenkins', passwordVariable: 'PASSWORD', usernameVariable: 'USERNAME')]) {
        sh '''
            set +x
            VERSION="$(cat version | head -n1 | sed 's/^[ \\ta-zA-Z$]*$/_/')"
            sudo docker login -u $USERNAME -p $PASSWORD panamax.spectrumxg.com
            sudo docker build --pull --no-cache=true -t ${DKR_IMAGE_NAME}:${DKR_IMAGE_LABEL} .
            sudo docker tag ${DKR_IMAGE_NAME}:${DKR_IMAGE_LABEL} ${DKR_IMAGE_NAME}:${DKR_IMAGE_LATEST_LABEL}
        '''
    }
    stage "Push Docker Image"
    withCredentials([usernamePassword(credentialsId: 'GitLabJenkins', passwordVariable: 'PASSWORD', usernameVariable: 'USERNAME')]) {
        sh '''
            set +x
            VERSION="$(cat version | head -n1 | sed 's/^[ \\ta-zA-Z$]*$//')"
            sudo docker push ${DKR_IMAGE_NAME}:${DKR_IMAGE_LABEL}
            sudo docker push ${DKR_IMAGE_NAME}:${DKR_IMAGE_LATEST_LABEL}
            sudo docker rmi ${DKR_IMAGE_NAME}:${DKR_IMAGE_LABEL}
            sudo docker images
        '''
    }
}
stage "Tagging Build"
node {
    withCredentials([usernameColonPassword(credentialsId: 'GitLabJenkins', variable: 'CREDS')]) {
        def fixedCreds = CREDS.replaceAll('@','%40')
        env.GIT_URL = gUrlClean.replaceAll('__GITAUTH__', fixedCreds)
        sh '''
            git config user.email "jenkins@noreply.org"
            git config user.name "jenkins"
            git tag -m "Successfully build." -a ${DKR_IMAGE_LABEL}
            git push --tags "${GIT_URL}"
        '''
    }
}
if(shouldDeploy) {
    stage "Deploying"
    node {
        env.DEPLOY_STACK_NAME = ("JNK-" + "${JOB_NAME}-${BRANCH_NAME}").replaceAll('[.*/]', '_')
        withCredentials([usernamePassword(credentialsId: 'RancherCredsAdmin', passwordVariable: 'PASSWORD', usernameVariable: 'USERNAME')]) {
            sh '''
            echo "COMPOSE_IMAGE_TAG=${DKR_IMAGE_LABEL}" > rancher_env_file
            echo "COMPOSE_HOST_LABEL=corp-route=true" >> rancher_env_file
            
            rancher --version
            rancher --debug --url "http://172.30.121.40:8080/v1" --access-key "$USERNAME" --secret-key "$PASSWORD" up --env-file rancher_env_file --pull --force-recreate --confirm-upgrade --stack ${DEPLOY_STACK_NAME} -d
        '''
        }
    }
}
if(allowTagLatest) {
    stage "Tag image as LATEST"
    input message: 'Tag this Docker Image as "latest."', ok: 'Tag'
    node {
        withCredentials([usernamePassword(credentialsId: 'GitLabJenkins', passwordVariable: 'PASSWORD', usernameVariable: 'USERNAME')]) {
            sh '''
            set +x
            VERSION="$(cat version | head -n1 | sed 's/^[ \\ta-zA-Z$]*$//')"
            sudo docker pull ${DKR_IMAGE_NAME}:${DKR_IMAGE_LABEL}
            sudo docker tag ${DKR_IMAGE_NAME}:${DKR_IMAGE_LABEL} ${DKR_IMAGE_NAME}:latest
            sudo docker push ${DKR_IMAGE_NAME}:latest
            sudo docker rmi ${DKR_IMAGE_NAME}:${DKR_IMAGE_LABEL}
            sudo docker images
        '''
        }
    }
}
