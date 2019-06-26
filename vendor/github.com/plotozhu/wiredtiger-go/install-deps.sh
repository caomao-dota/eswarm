#!/bin/sh

SHELL_FOLDER=$(pwd)
echo ${SHELL_FOLDER}
mkdir -p $SHELL_FOLDER/wdlibs/libs

cd $SHELL_FOLDER/wdlibs
if [ ! -d $SHELL_FOLDER/wdlibs/wiredtiger ]; then
  git clone https://github.com/wiredtiger/wiredtiger.git
  cd  wiredtiger

else
    cd  wiredtiger
    git pull https://github.com/wiredtiger/wiredtiger.git

fi


sh autogen.sh
./configure --prefix=$SHELL_FOLDER/wdlibs/libs 
make  && make install
