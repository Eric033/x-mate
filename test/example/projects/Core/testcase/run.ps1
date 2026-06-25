# 指定要遍历的目录路径
$directoryPath = "."  # 请将此路径替换为你的实际目录路径

# 获取目录中的所有XML文件
$xmlFiles = Get-ChildItem -Path $directoryPath -Filter "*.xml" -Recurse

foreach ($file in $xmlFiles) {
    try {
        # 读取XML文件内容
        [xml]$xmlContent = Get-Content -Path $file.FullName

        # 遍历XML文档中的所有case节点
        foreach ($caseNode in $xmlContent.SelectNodes("//case")) {
            # 提取trancode和tittle字段
            $trancode = $caseNode.guid
            $tittle = $caseNode.id
			
	
            $delimiters = @(',')  
			$array = $str.Split($delimiters, [StringSplitOptions]::RemoveEmptyEntries)
			$array | ForEach-Object { Write-Host "$trancode`t$_" } 

        }
    }
    catch {
        # 如果读取或解析XML文件时发生错误，输出错误信息
        Write-Error "Error processing file $($file.FullName): $_"
    }
}
