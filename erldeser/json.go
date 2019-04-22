package erldeser

import (
	"bytes"
	"fmt"
)

// WriteJSONToBuffer writes Erlang serialised JSON to given buffer
func (s *Scanner) WriteJSONToBuffer(collector *bytes.Buffer) error {
	err := s.readJSONValue(collector)
	if err != nil {
		slog.Error(err)
		return err
	}
	collector.WriteString("\n")
	return nil
}

// readJSONKeyValue reads JSON key-value pairs from Erlang serialised form
func (s *Scanner) readJSONKeyValue(collector *bytes.Buffer) error {
	t, err := s.Scan()
	if err != nil {
		slog.Error(err)
		return err
	}
	switch t.Term {
	case SmallTupleExt:
		// read key
		err := s.readJSONKey(collector)
		if err != nil {
			slog.Error(err)
			return err
		}
		// read value
		err = s.readJSONValue(collector)
		if err != nil {
			slog.Error(err)
			return err
		}
		return nil
	default:
		err := fmt.Errorf("Erlang serialised JSON key-value pair should be inside tuple, we got %v", t.Term)
		slog.Error(err)
		slog.Debug(collector.String())
		return err
	}
}

// readJSONKey is reading Erlang encoded JSON document key
func (s *Scanner) readJSONKey(collector *bytes.Buffer) error {
	t, err := s.Scan()
	if err != nil {
		slog.Error(err)
		return err
	}
	if t.Term != BinaryExt {
		err := fmt.Errorf("Erlang serialised JSON key should be binary, we got %v", t.Term)
		slog.Error(err)
		return err
	}
	_, err = collector.WriteString("\"")
	if err != nil {
		slog.Error(err)
		return err
	}
	_, err = collector.WriteString(string(t.Binary))
	if err != nil {
		slog.Error(err)
		return err
	}
	_, err = collector.WriteString("\":")
	if err != nil {
		slog.Error(err)
		return err
	}
	return nil
}

// readJSONValue is reading Erlang encoded JSON document value
func (s *Scanner) readJSONValue(collector *bytes.Buffer) error {
	t, err := s.Scan()
	if err != nil {
		slog.Error(err)
		return err
	}
	switch t.Term {
	case NewFloatExt:
		_, err = collector.WriteString(fmt.Sprintf("%v", t.FloatValue))
		if err != nil {
			slog.Error(err)
			return err
		}
	case SmallIntegerExt:
		_, err = collector.WriteString(fmt.Sprintf("%v", t.IntegerValue))
		if err != nil {
			slog.Error(err)
			return err
		}
	case IntegerExt:
		_, err = collector.WriteString(fmt.Sprintf("%v", t.IntegerValue))
		if err != nil {
			slog.Error(err)
			return err
		}
	case AtomExt:
		_, err = collector.WriteString(fmt.Sprintf("%v", t.StringValue))
		if err != nil {
			slog.Error(err)
			return err
		}
	case SmallTupleExt:
		t, err := s.Scan()
		if err != nil {
			slog.Error(err)
			return err
		}
		switch t.Term {
		case ListExt:
			_, err = collector.WriteString("{")
			if err != nil {
				slog.Error(err)
				return err
			}
			// For each element in the list
			for i := int64(0); i < t.IntegerValue; i++ {
				err := s.readJSONKeyValue(collector)
				if err != nil {
					slog.Error(err)
					return err
				}
				if i < t.IntegerValue-1 {
					_, err = collector.WriteString(",")
					if err != nil {
						slog.Error(err)
						return err
					}
				}
			}
			_, err = collector.WriteString("}")
			if err != nil {
				slog.Error(err)
				return err
			}
			return nil
		default:
			err := fmt.Errorf("Erlang serialised JSON object should start as tuple containing list, we got %v", t.Term)
			slog.Error(err)
			return err
		}
	case NilExt:
		_, err = collector.WriteString("null")
		if err != nil {
			slog.Error(err)
			return err
		}
	case StringExt:
		// Actually array of small integers!!
		_, err = collector.WriteString("[")
		if err != nil {
			slog.Error(err)
			return err
		}
		l := len(t.Binary)
		for i := 0; i < l; i++ {
			_, err = collector.WriteString(fmt.Sprintf("%d", t.Binary[i]))
			if err != nil {
				slog.Error(err)
				return err
			}
			if i < l-1 {
				_, err = collector.WriteString(",")
				if err != nil {
					slog.Error(err)
					return err
				}
			}
		}
		_, err = collector.WriteString("]")
		if err != nil {
			slog.Error(err)
			return err
		}
	case ListExt:
		_, err = collector.WriteString("[")
		if err != nil {
			slog.Error(err)
			return err
		}
		for i := int64(0); i < t.IntegerValue; i++ {
			err = s.readJSONValue(collector)
			if err != nil {
				slog.Error(err)
				return err
			}
			if i < t.IntegerValue-1 {
				_, err = collector.WriteString(",")
				if err != nil {
					slog.Error(err)
					return err
				}
			}
		}
		t, err := s.Scan()
		if err != nil {
			slog.Error(err)
			return err
		}
		if t.Term != NilExt {
			err = fmt.Errorf("Erlang serialised list should end with extra nil, but ends with %v", t.Term)
			slog.Error(err)
			return err
		}
		_, err = collector.WriteString("]")
		if err != nil {
			slog.Error(err)
			return err
		}
	case BinaryExt:
		_, err = collector.WriteString(fmt.Sprintf("\"%s\"", string(t.Binary)))
		if err != nil {
			slog.Error(err)
			return err
		}
	default:
		err = fmt.Errorf("Don't know how to turn type %v into JSON value", t.Term)
		slog.Error(err)
		return err
	}
	if err != nil {
		slog.Error(err)
		return err
	}
	return nil
}