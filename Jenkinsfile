pipeline {
    agent any
    environment {
        JOB_NAME = 'e2e-resourcelimiter-test'
    }
    stages {      
        stage('Checkout Codes') {
            steps {
                echo 'Checkout codes'
                checkout([$class: 'GitSCM',
                    branches: [[name: '${GITHUB_PR_SOURCE_BRANCH}']],
                    userRemoteConfigs: [[credentialsId:  'dce2dba9-82cc-4355-9a92-f5dc2049b45b', url: 'git@github.com:chenliu1993/resourcelimiter.git']]])
                    // sh 'git checkout ${GITHUB_PR_SOURCE_BRANCH}'
            }
        }
        stage('Start') {
            steps {
                echo 'Start'
            }
        }
        stage('Build') {
            steps {
                echo 'Build a new controller bin to test'
                sh 'make build'
            }
        }

        stage('Prepare KinD Cluster') {
            steps {
                echo 'Prepare the kind cluster'
                sh 'kind create cluster --config ~/kind-config.yaml'
            }
        }
        stage('Start Controller E2E Test') {
            steps {
                echo 'run controller tests, currently there is only controller'
                sh 'make e2e-test'
            }
        }
    }
    post {
        success {
            echo 'E2E tests succeed, clean up the environment'
            setGitHubPullRequestStatus context: 'e2e-resourcelimiter-test', message: 'E2E test succeed', state: 'SUCCESS' 
            githubPRComment comment: githubPRMessage('Controller E2E Test Success.'), statusVerifier: allowRunOnStatus("SUCCESS"), errorHandler: statusOnPublisherError("UNSTABLE")
        }
        always {
            sh 'kind delete cluster'
            deleteDir()
        }
        failure {
            echo 'E2E tests failed, clean up the environment'
            setGitHubPullRequestStatus context: 'e2e-resourcelimiter-test', message: 'E2E test failed', state: 'FAILURE'
            githubPRComment comment: githubPRMessage('Controller E2E Test failed.'), statusVerifier: allowRunOnStatus("FAILURE"), errorHandler: statusOnPublisherError("UNSTABLE")
        }        
    }
}