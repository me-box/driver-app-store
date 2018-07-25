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

- Add and interface to list manifests
- Add the ability to upload local manifests (maybe via rpc api?)
- Add the ability to add new data stores (maybe via rpc api?)
