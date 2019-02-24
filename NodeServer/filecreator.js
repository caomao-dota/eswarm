var program = require('commander');
var fs = require('fs')
var dm = require('./dirman')

const stringRandom = require('string-random');
function list(val) {
    return val.split('..').map(Number);
    }
function main(){
    program
    .version('0.1.0')
    .option('-s, --size [value]', '文件大小, 仅有三个选项 s 1MB m:10MB l:100MB',"s")
    .option('-a, --amount <n>', '一次创建多少个文件',parseInt)
    .option('-i, --index <n>', '文件名从哪个开始  文件名总是datx的形式，如dat1...dat10等等',parseInt)
    .option('-d, --dir [value]','存放生成文件的父目录，大文件存放于 dir/large , 中文件存放于 dir/middle ， 小文件存放于 dir/small',".")
    
    program.parse(process.argv);
    console.log(' size - %s ', program.size);
    console.log(' amount: %j ', program.amount);
    console.log(' index: %j', program.index);
    console.log(' dir -s ', program.dir);


    size = (program.size && program.size.toLowerCase() ) || 's'
    if (size != 'm' && size != 'l'){
        size = 's'
    }

    amount = (program.amount && parseInt(program.amount)) ||2

    start = (program.index && parseInt(program.index)) || 1

    dir = (program.dir || ".")
    createFiles(dir,size,amount,start)
}
function createFiles(dir,size,amount,start){
    dirTemp ={
        "s":{
            dir:"small",
            size:1
        },
        "m":{
            dir:"middle",
            size:10
        },
        "l":{
            dir:"large",
            size:100
        },
    }
    //创建目录
    targetDir = dir+"/"+dirTemp[size].dir+"/"
    dm.mkdirsSync(targetDir)
    for(i = 0; i < amount; i++){
        b = i
        console.log("正在创建",b+1,"/",amount)
        createOneFile(targetDir+"dat"+(start+b),dirTemp[size].size)
        console.log("")
    }
    console.log("文件创建已经完成，请查看目录：",targetDir)
}
function createOneFile(filename,filesize){
    totalIndice = filesize* 10
   
    try{
        fs.unlinkSync(filename)
    }    catch(e){

    }
    for (j = 0; j < totalIndice; j++){
        process.stdout.write("#")
        newBuffer =  stringRandom(102400, {specials: true}); // ,o=8l{iay>AOegW[
        fs.appendFileSync(filename,newBuffer)
    }
    
}
console.log("hello")
main()
