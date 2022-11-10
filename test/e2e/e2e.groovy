pipeline {
    agent any
    stages {      
        stage('Start') {
            steps {
                echo 'Start E2E tests'
            }
        }
        stage('Build') {
            steps {
                echo 'build controller...'
                sh 'make build'
            }
        }
        stage('E2E Test') {
            steps {
                echo 'set up environments for tests'
                sh 'kind delete cluster; kind create cluster --config /disk1/cliu/kind-config.yaml; sleep 60s;'
                echo 'run E2E tests, this is the same tests set as the main branch'
                sh 'make e2e-test'
            }
        }
    }
    post {
        always {
            script {
                echo 'Finished'
            }
        }
        
    }
}