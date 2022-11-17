package tag

import (
	"context"

	"github.com/answerdev/answer/internal/base/data"
	"github.com/answerdev/answer/internal/base/pager"
	"github.com/answerdev/answer/internal/base/reason"
	"github.com/answerdev/answer/internal/entity"
	"github.com/answerdev/answer/internal/service/siteinfo_common"
	tagcommon "github.com/answerdev/answer/internal/service/tag_common"
	"github.com/answerdev/answer/internal/service/unique"
	"github.com/segmentfault/pacman/errors"
	"github.com/segmentfault/pacman/log"
	"xorm.io/builder"
)

// tagRepo tag repository
type tagRepo struct {
	data            *data.Data
	uniqueIDRepo    unique.UniqueIDRepo
	siteInfoService *siteinfo_common.SiteInfoCommonService
}

// NewTagRepo new repository
func NewTagRepo(
	data *data.Data,
	uniqueIDRepo unique.UniqueIDRepo,
	siteInfoService *siteinfo_common.SiteInfoCommonService,

) tagcommon.TagRepo {
	return &tagRepo{
		data:            data,
		uniqueIDRepo:    uniqueIDRepo,
		siteInfoService: siteInfoService,
	}
}

func (tr *tagRepo) tagRecommendStatus(ctx context.Context) bool {
	tagconfig, err := tr.siteInfoService.GetSiteWrite(ctx)
	if err != nil {
		log.Error("siteInfoService.GetSiteWrite error", err)
		return false
	}
	return tagconfig.RequiredTag
}

// AddTagList add tag
func (tr *tagRepo) AddTagList(ctx context.Context, tagList []*entity.Tag) (err error) {
	for _, item := range tagList {
		item.ID, err = tr.uniqueIDRepo.GenUniqueIDStr(ctx, item.TableName())
		if err != nil {
			return err
		}
		item.RevisionID = "0"
	}
	_, err = tr.data.DB.Insert(tagList)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	return
}

// GetTagListByIDs get tag list all
func (tr *tagRepo) GetTagListByIDs(ctx context.Context, ids []string) (tagList []*entity.Tag, err error) {
	tagList = make([]*entity.Tag, 0)
	session := tr.data.DB.In("id", ids)
	session.Where(builder.Eq{"status": entity.TagStatusAvailable})
	err = session.OrderBy("recommend desc,reserved desc,id desc").Find(&tagList)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	if !tr.tagRecommendStatus(ctx) {
		for _, tag := range tagList {
			tag.Recommend = false
		}
	}
	return
}

// GetTagBySlugName get tag by slug name
func (tr *tagRepo) GetTagBySlugName(ctx context.Context, slugName string) (tagInfo *entity.Tag, exist bool, err error) {
	tagInfo = &entity.Tag{}
	session := tr.data.DB.Where("slug_name = ?", slugName)
	session.Where(builder.Eq{"status": entity.TagStatusAvailable})
	exist, err = session.Get(tagInfo)
	if err != nil {
		return nil, false, errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}

	if !tr.tagRecommendStatus(ctx) {
		tagInfo.Recommend = false
	}

	return
}

// GetTagListByName get tag list all like name
func (tr *tagRepo) GetTagListByName(ctx context.Context, name string, limit int, hasReserved bool) (tagList []*entity.Tag, err error) {
	tagList = make([]*entity.Tag, 0)
	cond := &entity.Tag{}
	session := tr.data.DB.Where("")
	if name != "" {
		session.Where("slug_name LIKE ?", name+"%")
	} else {
		cond.Recommend = true
	}
	session.Where(builder.Eq{"status": entity.TagStatusAvailable})
	session.Limit(limit).Asc("slug_name")
	if !hasReserved {
		cond.Reserved = false
		session.UseBool("recommend", "reserved")
	} else {
		session.UseBool("recommend")
	}
	err = session.OrderBy("recommend desc,reserved desc,id desc").Find(&tagList, cond)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	if !tr.tagRecommendStatus(ctx) {
		for _, tag := range tagList {
			tag.Recommend = false
		}
	}
	return
}

func (tr *tagRepo) GetRecommendTagList(ctx context.Context) (tagList []*entity.Tag, err error) {
	tagList = make([]*entity.Tag, 0)
	cond := &entity.Tag{}
	session := tr.data.DB.Where("")
	cond.Recommend = true
	// session.Where(builder.Eq{"status": entity.TagStatusAvailable})
	session.Asc("slug_name")
	session.UseBool("recommend")
	err = session.Find(&tagList, cond)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	if !tr.tagRecommendStatus(ctx) {
		for _, tag := range tagList {
			tag.Recommend = false
		}
	}
	return
}

func (tr *tagRepo) GetReservedTagList(ctx context.Context) (tagList []*entity.Tag, err error) {
	tagList = make([]*entity.Tag, 0)
	cond := &entity.Tag{}
	session := tr.data.DB.Where("")
	cond.Reserved = true
	// session.Where(builder.Eq{"status": entity.TagStatusAvailable})
	session.Asc("slug_name")
	session.UseBool("reserved")
	err = session.Find(&tagList, cond)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	if !tr.tagRecommendStatus(ctx) {
		for _, tag := range tagList {
			tag.Recommend = false
		}
	}
	return
}

// GetTagListByNames get tag list all like name
func (tr *tagRepo) GetTagListByNames(ctx context.Context, names []string) (tagList []*entity.Tag, err error) {
	tagList = make([]*entity.Tag, 0)
	session := tr.data.DB.In("slug_name", names).UseBool("recommend", "reserved")
	// session.Where(builder.Eq{"status": entity.TagStatusAvailable})
	err = session.OrderBy("recommend desc,reserved desc,id desc").Find(&tagList)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	if !tr.tagRecommendStatus(ctx) {
		for _, tag := range tagList {
			tag.Recommend = false
		}
	}
	return
}

// RemoveTag delete tag
func (tr *tagRepo) RemoveTag(ctx context.Context, tagID string) (err error) {
	session := tr.data.DB.Where(builder.Eq{"id": tagID})
	_, err = session.Update(&entity.Tag{Status: entity.TagStatusDeleted})
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	return
}

// UpdateTag update tag
func (tr *tagRepo) UpdateTag(ctx context.Context, tag *entity.Tag) (err error) {
	_, err = tr.data.DB.Where(builder.Eq{"id": tag.ID}).Update(tag)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	return
}

// UpdateTagQuestionCount update tag question count
func (tr *tagRepo) UpdateTagQuestionCount(ctx context.Context, tagID string, questionCount int) (err error) {
	cond := &entity.Tag{QuestionCount: questionCount}
	_, err = tr.data.DB.Where(builder.Eq{"id": tagID}).MustCols("question_count").Update(cond)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	return
}

// UpdateTagSynonym update synonym tag
func (tr *tagRepo) UpdateTagSynonym(ctx context.Context, tagSlugNameList []string, mainTagID int64,
	mainTagSlugName string,
) (err error) {
	bean := &entity.Tag{MainTagID: mainTagID, MainTagSlugName: mainTagSlugName}
	session := tr.data.DB.In("slug_name", tagSlugNameList).MustCols("main_tag_id", "main_tag_slug_name")
	_, err = session.Update(bean)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	return
}

func (tr *tagRepo) UpdateTagsAttribute(ctx context.Context, tags []string, attribute string, value bool) (err error) {
	bean := &entity.Tag{}
	switch attribute {
	case "recommend":
		bean.Recommend = value
	case "reserved":
		bean.Reserved = value
	default:
		return
	}
	session := tr.data.DB.In("slug_name", tags).Cols(attribute).UseBool(attribute)
	_, err = session.Update(bean)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	return
}

// GetTagByID get tag one
func (tr *tagRepo) GetTagByID(ctx context.Context, tagID string) (
	tag *entity.Tag, exist bool, err error,
) {
	tag = &entity.Tag{}
	session := tr.data.DB.Where(builder.Eq{"id": tagID})
	session.Where(builder.Eq{"status": entity.TagStatusAvailable})
	exist, err = session.Get(tag)
	if err != nil {
		return nil, false, errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	if !tr.tagRecommendStatus(ctx) {
		tag.Recommend = false
	}
	return
}

// GetTagList get tag list all
func (tr *tagRepo) GetTagList(ctx context.Context, tag *entity.Tag) (tagList []*entity.Tag, err error) {
	tagList = make([]*entity.Tag, 0)
	session := tr.data.DB.Where(builder.Eq{"status": entity.TagStatusAvailable})
	err = session.Find(&tagList, tag)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	if !tr.tagRecommendStatus(ctx) {
		for _, tag := range tagList {
			tag.Recommend = false
		}
	}
	return
}

// GetTagPage get tag page
func (tr *tagRepo) GetTagPage(ctx context.Context, page, pageSize int, tag *entity.Tag, queryCond string) (
	tagList []*entity.Tag, total int64, err error,
) {
	tagList = make([]*entity.Tag, 0)
	session := tr.data.DB.NewSession()

	if len(tag.SlugName) > 0 {
		session.Where(builder.Or(builder.Like{"slug_name", tag.SlugName}, builder.Like{"display_name", tag.SlugName}))
		tag.SlugName = ""
	}
	session.Where(builder.Eq{"status": entity.TagStatusAvailable})
	session.Where("main_tag_id = 0") // if this tag is synonym, exclude it

	switch queryCond {
	case "popular":
		session.Desc("question_count")
	case "name":
		session.Asc("slug_name")
	case "newest":
		session.Desc("created_at")
	}

	total, err = pager.Help(page, pageSize, &tagList, tag, session)
	if err != nil {
		err = errors.InternalServer(reason.DatabaseError).WithError(err).WithStack()
	}
	if !tr.tagRecommendStatus(ctx) {
		for _, tag := range tagList {
			tag.Recommend = false
		}
	}
	return
}
