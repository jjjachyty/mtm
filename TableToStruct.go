package mtm

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

//参考
//https://blog.csdn.net/Charles_Thanks/article/details/80503124

// map for converting mysql type to golang types
var typeForMysqlToGo = map[string]string{
	"int":                "int32",
	"integer":            "int32",
	"tinyint":            "int32",
	"smallint":           "int32",
	"mediumint":          "int32",
	"bigint":             "int64",
	"int unsigned":       "uint32",
	"integer unsigned":   "uint32",
	"tinyint unsigned":   "uint32",
	"smallint unsigned":  "uint32",
	"mediumint unsigned": "uint32",
	"bigint unsigned":    "uint64",
	"decimal unsigned":   "uint64",
	"bit":                "int",
	"bool":               "bool",
	"enum":               "string",
	"set":                "string",
	"varchar":            "string",
	"char":               "string",
	"tinytext":           "string",
	"mediumtext":         "string",
	"text":               "string",
	"longtext":           "string",
	"blob":               "string",
	"tinyblob":           "string",
	"mediumblob":         "string",
	"longblob":           "string",
	"date":               "time.Time", // time.Time or string
	"datetime":           "string",    // time.Time or string
	"timestamp":          "time.Time", // time.Time or string
	"time":               "time.Time", // time.Time or string
	"float":              "float32",
	"double":             "float64",
	"decimal":            "float64",
	"binary":             "string",
	"varbinary":          "string",
	"json":               "string",
}

func CreateTableToStruct(options *Options) *TableToStruct {
	if len(options.MySqlUrl) == 0 {
		log.Fatal("MySqlUrl参数不能为空")
	}
	if len(options.PackageName) == 0 {
		options.PackageName = "entity"
	}
	if len(options.SavePath) == 0 {
		options.SavePath = "./Models"
	}
	if len(options.FileName) == 0 {
		options.FileName = "Models.go"
	}

	t2s := new(TableToStruct)
	if options != nil {
		t2s.MySqlUrl = options.MySqlUrl
		t2s.IfOneFile = options.IfOneFile
		t2s.FileName = options.FileName
		t2s.PackageName = options.PackageName
		t2s.SavePath = options.SavePath
		t2s.IfToHump = options.IfToHump
		t2s.IfJsonTag = options.IfJsonTag
		t2s.IfPluralToSingular = options.IfPluralToSingular
		t2s.IfCapitalizeFirstLetter = options.IfCapitalizeFirstLetter
	}
	return t2s
}
func (t2s *TableToStruct) Run(pbUrl string) error {
	//1、获取table列表
	db, err := CreateMysqlDb(t2s.MySqlUrl)
	if err != nil {
		return err
	}
	tables, err := db.Query("SELECT table_schema,table_name FROM information_schema.TABLES WHERE table_schema=DATABASE () AND table_name not like 'QRTZ%' AND table_type='BASE TABLE'; ")
	if err != nil {
		return err
	}
	defer tables.Close()

	for tables.Next() {
		tableSchema := ""
		structName := ""

		err = tables.Scan(&tableSchema, &structName)
		if err != nil {
			return err
		}
		ttf := new(TableToFile)
		ttf._import = make(map[string]string)
		ttf._struct = structName

		//2、循环获取table Column列表
		columns, err := db.Query("SELECT COLUMN_NAME,DATA_TYPE,COLUMN_TYPE,IS_NULLABLE,TABLE_NAME,COLUMN_COMMENT FROM information_schema.COLUMNS WHERE table_schema=DATABASE () AND table_name=? order by ORDINAL_POSITION;", structName)
		if err != nil {
			return err
		}
		defer columns.Close()
		//3、打印输出格式
		//3.1、输出类名
		if t2s.IfToHump {
			structName = toHump(structName)
		}
		if t2s.IfPluralToSingular {
			structName = toSingular(structName)
		}
		if t2s.IfCapitalizeFirstLetter {
			structName = strFirstToUpper(structName)
		} else {
			structName = strFirstToLower(structName)
		}
		if t2s.IfCapitalizeFirstLetter {
			structName = strFirstToUpper(structName)
		}

		ttf._fileName = strFirstToLower(structName)
		ttf._comment = "//" + structName + "\n"
		ttf._struct = "type " + structName + " struct {\n"
		//3.2、输出属性
		ttf._property = make([]string, 0)
		urls := strings.Split(pbUrl, " ")
		alias := urls[0]
		ttf._import[alias] = pbUrl

		funcPb := "func (v *" + structName + ")" + "Pb" + "()" + "*" + alias + "." + structName + "{" + "\n" +
			"	return" + " &" + alias + "." + structName + "{\n"
		funcPbs := "type " + structName + "s  []*" + structName + "\n\n" +
			"func (v " + structName + "s" + ")" + "Pbs" + "()" + "(data []*" + alias + "." + structName + "){" + "\n" + "" +
			"   data = make([]*" + alias + "." + structName + "," + "0) \n" +
			"	for _,v := range v { \n" +
			"		data = append(data," + "&" + alias + "." + structName + "{\n"

		funcEntitys := "type " + structName + "Pbs  []*" + alias + "." + structName + "\n\n" +
			"func (v " + structName + "Pbs" + ")" + "Entitys" + "()" + "(data []*" + structName + "){" + "\n" + "" +
			"   data = make([]*" + structName + "," + "0) \n" +
			"	for _,v := range v { \n" +
			"		data = append(data,&" + structName + "{\n"

		funcEn := "func (v *" + structName + ")Pb2Entity" + "(data *" + alias + "." + structName + "){" + "\n" +
			"if data == nil{return} \n"
		for columns.Next() {
			columnName := ""
			dataType := ""
			isNullable := ""
			tableName := ""
			columnType := ""
			columnComment := ""
			err = columns.Scan(&columnName, &dataType, &columnType, &isNullable, &tableName, &columnComment)
			if err != nil {
				return err
			}
			//_type := ""
			//var ok bool
			//if strings.Contains(columnType, "unsigned") {
			//	_type, ok = typeForMysqlToGo[columnType]
			//	if !ok {
			//		_type = "[]byte"
			//	}
			//} else {
			//	_type, ok = typeForMysqlToGo[dataType]
			//	if !ok {
			//		_type = "[]byte"
			//	}
			//}
			_type, ok := typeForMysqlToGo[dataType]
			if !ok {
				_type = "[]byte"
			}
			if _type == "time.Time" {
				ttf._import["time"] = `"time"`
			}
			columnName2 := ""

			if t2s.IfCapitalizeFirstLetter {
				columnName2 = strFirstToUpper(columnName)
			} else {
				columnName2 = strFirstToLower(columnName)
			}

			if t2s.IfToHump {
				columnName2 = toHump(columnName2)
			}
			if t2s.IfCapitalizeFirstLetter {
				columnName2 = strFirstToUpper(columnName2)
			} else {
				columnName2 = strFirstToLower(columnName2)
			}

			ttf._property = append(ttf._property, fmt.Sprintf("	%s %s `db:\"%s\" json:\"%s\" ` //%s", columnName2, _type, columnName, strFirstToLower(columnName2), columnComment))

			funcPb += "			" + columnName2 + ":" + "v." + columnName2 + ",\n"
			funcPbs += "		" + columnName2 + ":" + "v." + columnName2 + ",\n"
			funcEntitys += "		" + columnName2 + ":" + "v." + columnName2 + ",\n"
			funcEn += "         " + "v." + columnName2 + " = " + "data." + columnName2 + "\n"
			if t2s.IfToHump {
				columnName = toHump(columnName)
			}

			if t2s.IfCapitalizeFirstLetter {
				columnName = strFirstToUpper(columnName)
			} else {
				columnName = strFirstToLower(columnName)
			}

			if t2s.IfCapitalizeFirstLetter {
				columnName = strFirstToUpper(columnName)
			} else {
				columnName = strFirstToLower(columnName)
			}
		}
		funcPb += "}\n	}\n\n"
		funcPbs += "})\n	}\n return \n}\n\n"
		funcEn += "}\n\n	"
		funcEntitys += "})\n	}\n return \n}\n\n"
		ttf._func = funcPb + funcPbs + funcEn + funcEntitys
		t2s.tableToFile = append(t2s.tableToFile, ttf)
	}
	//4、写入文件
	err = t2s.saveToFile()
	if err != nil {
		log.Fatal(err.Error())
	}
	cmd := exec.Command("gofmt", "-w", t2s.SavePath)
	cmd.Run()
	log.Print("模型装换成功")
	return nil
}

type Options struct {
	MySqlUrl                string   //数据库地址 DSN (Data Source Name) ：[username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
	IfOneFile               bool     //多个表是否放在同一文件 true=同一文件 默认false
	FileName                string   //文件名 当IfOneFile=true时有效 默认Models.go
	PackageName             string   //自定义项目package名称 默认Models
	SavePath                string   //保存文件夹 默认./Models
	IfToHump                bool     //是否转换驼峰 true=是 默认false
	IfJsonTag               bool     //是否包含json tag true=是 默认false
	IfPluralToSingular      bool     //是否复数转单数 true=是 默认false
	IfCapitalizeFirstLetter bool     //是否首字母转换大写 true=是 默认false
	IfFunc                  bool     //是否生成Pbs
	FuncImport              []string //引用的包路径
}
type TableToStruct struct {
	MySqlUrl                string
	IfOneFile               bool
	FileName                string
	PackageName             string
	SavePath                string
	IfToHump                bool
	IfJsonTag               bool
	IfPluralToSingular      bool
	IfCapitalizeFirstLetter bool
	tableToFile             []*TableToFile
	Comment                 string
	IfFunc                  bool
	FuncImport              []string
}
type TableToFile struct {
	_import   map[string]string
	_struct   string
	_fileName string
	_property []string
	_comment  string
	_func     string
}

func (t *TableToFile) _importToStr() string {
	im := ""
	for _, value := range t._import {
		im += value + "\n"
	}
	return im
}
func (t *TableToFile) _propertyToStr() string {
	return strings.Join(t._property, "\n")
}

func (t *TableToStruct) saveToFile() error {
	if !t.IfOneFile {
		for _, v := range t.tableToFile {
			//4、写入文件
			file := "package " + strings.ToLower(t.PackageName) + "\n" + "import (\n" + v._importToStr() + ")\n" + v._comment + v._struct + v._propertyToStr() + "\n}\n" + v._func
			err := t.save(v._fileName+".go", file)
			if err != nil {
				return err
			}
		}
	} else {
		content := ""
		importStr := "import (\n"
		for _, v := range t.tableToFile {
			//4、写入文件
			importStr += v._importToStr()
			content += v._comment + v._struct + v._propertyToStr() + "\n}\n"
		}
		importStr += ")\n"

		file := "package " + t.PackageName + "\n" + importStr + content
		err := t.save(t.FileName, file)
		if err != nil {
			return err
		}
	}
	return nil
}
func (t *TableToStruct) save(fileName string, content string) error {
	//4、写入文件
	//容错
	if t.SavePath[len(t.SavePath)-1] != '/' {
		t.SavePath += "/"
	}
	//创建目录
	os.MkdirAll(t.SavePath, 0777)
	//创建文件a
	filePath := t.SavePath + fileName
	f, err := os.Create(filePath)
	defer f.Close()
	if err != nil {
		return err
	}
	f.WriteString(content)
	return nil

}

// Convert The First Letter To Capitalize
func strFirstToUpper(str string) string {
	if len(str) < 1 {
		return ""
	}
	//if unicode.IsUpper([]rune(str)[0]) {
	//	return str
	//}
	strArry := []rune(str)
	if strArry[0] >= 97 && strArry[0] <= 122 {
		strArry[0] -= 32
	}
	return string(strArry)
}

// Convert The First Letter To Capitalize
func strFirstToLower(str string) string {
	if len(str) < 1 {
		return ""
	}
	if str == "ID" {
		return "id"
	}
	//if unicode.IsLower([]rune(str)[0]) {
	//	return str
	//}
	strArry := []rune(str)
	if strArry[0] >= 65 && strArry[0] <= 90 {
		strArry[0] += 32
	}
	return string(strArry)
}

// Convert The Plural To Singular
func toSingular(word string) string {
	plural1, _ := regexp.Compile("([^aeiou])ies$")
	plural2, _ := regexp.Compile("([aeiou]y)s$")
	plural3, _ := regexp.Compile("([sxzh])es$")
	plural4, _ := regexp.Compile("([^sxzhyu])s$")
	if plural1.Match([]byte(word)) {
		return word[0:len(word)-3] + "y"
	} else if plural2.Match([]byte(word)) {
		return word[0 : len(word)-1]
	} else if plural3.Match([]byte(word)) {
		return word[0 : len(word)-2]
	} else if plural4.Match([]byte(word)) {
		return word[0 : len(word)-1]
	}
	return word
}

// 转换驼峰 id =>ID
func toHump(c string) string {
	cg := strings.Split(c, "_")
	p := ""
	for _, v := range cg {
		if v == "Id" || v == "id" {
			p += "ID"
			continue
		}
		p += strFirstToUpper(v)
	}
	return p
}
