# databox-app-store-driver

The official databox app store driver, reads databox manifests from a git repository stored on git hub and writes them into a local databox store for access by other components.

This driver registers two key value data sources, one for apps (type databox:manifests:app) and one for drivers (type databox:manifests:driver). The keys are the app/driver name and the value is the json representation of the corresponding manifest.

## Testing/developing outside databox

Its passable to test this component outside of databox, to do so run:

```
    ./setupTests.sh
    go run *.go -giturl https://github.com/Toshbrown/databox-manifest-store --storeurl tcp://127.0.0.1:5555 --arbiterurl tcp://127.0.0.1:4444
```

# TODO

- Add the ability to add new manifest stores

## Development of databox was supported by the following funding

```
EP/N028260/1, Databox: Privacy-Aware Infrastructure for Managing Personal Data

EP/N028260/2, Databox: Privacy-Aware Infrastructure for Managing Personal Data

EP/N014243/1, Future Everyday Interaction with the Autonomous Internet of Things

EP/M001636/1, Privacy-by-Design: Building Accountability into the Internet of Things (IoTDatabox)

EP/M02315X/1, From Human Data to Personal Experience

```