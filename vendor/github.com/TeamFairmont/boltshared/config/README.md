#config

The package provides the following functions:
* DefaultConfig: Returns a config object populated with the default settings.
* CustomizeConfig: Takes a Config object (populated with config_defaults.json) and a string of json (custom settings read from config.json).  Returns the config after overwriting any matching settings from the string of json.

If you need to override a setting, edit /etc/bolt/config.json
The /etc/bolt/config.json file should have been created as part of the initial bolt setup, as specified in the boltengine's top level README.md
