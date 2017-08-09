# Route-service-static-broker

A cloud foundry service broker to create static route service without pain.

## Installation

1. Create an user provided service to manage your configuration, you can modified the file `user-provided-service.json` and run `cf cups <my-route-service>-config -p user-provided-service.json`
2. Modify the `manifest.yml` file (don't forget to use your previously created cups)
3. Run `cf push`

 