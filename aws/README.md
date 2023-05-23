# xcAwsInventory.py
generate xcdata.ini from aws ec2

## dependencies
* python3
* aws cli

## usage
just run xcAwsInventory.py

## settings
config files paths: /etc/xcAwsInventory/config.yaml, ~/.xcAwsInventory.yaml
default settings:
```
logFile: 'stdout'
logLevel: 'info'
regions: [] # all regions
iniFilePath: '~/xcdata.ini' # path to result ini file
tagForMainGroup: 'Name' # tag 'Key' for 'mainGroup'
tagForParentGroup: 'role' # tag 'Key' for 'parentGroup'
```
