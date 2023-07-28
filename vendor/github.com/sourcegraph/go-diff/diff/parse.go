	return &MultiFileDiffReader{reader: bufio.NewReader(r)}
	reader *bufio.Reader
				return nil, io.EOF
			return nil, err
			return fd, nil
			return nil, err
		return fd, nil
	line, err := readLine(r.reader)
		return fd, err
					return fd, nil
			return nil, err
	return fd, nil
	return &FileDiffReader{reader: bufio.NewReader(r)}
	reader *bufio.Reader

		line, err = readLine(r.reader)
		ts, err := time.Parse(diffTimeParseLayout, parts[1])
			return "", nil, err
			line, err = readLine(r.reader)
	var err error
	switch {
	case (lineCount == 3 || lineCount == 4 && strings.HasPrefix(fd.Extended[3], "Binary files ") || lineCount > 4 && strings.HasPrefix(fd.Extended[3], "GIT binary patch")) &&
		strings.HasPrefix(fd.Extended[1], "new file mode "):
		names := strings.SplitN(fd.Extended[0][len("diff --git "):], " ", 2)
		fd.NewName, err = strconv.Unquote(names[1])
		if err != nil {
			fd.NewName = names[1]
		}
		return true
	case (lineCount == 3 || lineCount == 4 && strings.HasPrefix(fd.Extended[3], "Binary files ") || lineCount > 4 && strings.HasPrefix(fd.Extended[3], "GIT binary patch")) &&
		strings.HasPrefix(fd.Extended[1], "deleted file mode "):
		names := strings.SplitN(fd.Extended[0][len("diff --git "):], " ", 2)
		fd.OrigName, err = strconv.Unquote(names[0])
		if err != nil {
			fd.OrigName = names[0]
		}
		return true
	case lineCount == 4 && strings.HasPrefix(fd.Extended[2], "rename from ") && strings.HasPrefix(fd.Extended[3], "rename to "):
		names := strings.SplitN(fd.Extended[0][len("diff --git "):], " ", 2)
		fd.OrigName, err = strconv.Unquote(names[0])
		if err != nil {
			fd.OrigName = names[0]
		fd.NewName, err = strconv.Unquote(names[1])
		if err != nil {
			fd.NewName = names[1]
		return true
	case lineCount == 6 && strings.HasPrefix(fd.Extended[5], "Binary files ") && strings.HasPrefix(fd.Extended[2], "rename from ") && strings.HasPrefix(fd.Extended[3], "rename to "):
		names := strings.SplitN(fd.Extended[0][len("diff --git "):], " ", 2)
		fd.OrigName = names[0]
		fd.NewName = names[1]
		return true
	case lineCount == 3 && strings.HasPrefix(fd.Extended[2], "Binary files ") || lineCount > 3 && strings.HasPrefix(fd.Extended[2], "GIT binary patch"):
		names := strings.SplitN(fd.Extended[0][len("diff --git "):], " ", 2)
		fd.OrigName, err = strconv.Unquote(names[0])
		if err != nil {
			fd.OrigName = names[0]
		}
		fd.NewName, err = strconv.Unquote(names[1])
		if err != nil {
			fd.NewName = names[1]
		}
		return true
	default:
		return false
	return &HunksReader{reader: bufio.NewReader(r)}
	reader *bufio.Reader
			line, err = readLine(r.reader)
				ok, err := peekPrefix(r.reader, "+++")
					return r.hunk, &ParseError{r.line, r.offset, &ErrBadHunkLine{Line: line}}
// peekPrefix peeks into the given reader to check whether the next
// bytes match the given prefix.
func peekPrefix(reader *bufio.Reader, prefix string) (bool, error) {
	next, err := reader.Peek(len(prefix))
	if err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	return bytes.HasPrefix(next, []byte(prefix)), nil
}
