package activity

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/answerdev/answer/internal/base/constant"
	"github.com/answerdev/answer/internal/entity"
	"github.com/answerdev/answer/internal/repo/config"
	"github.com/answerdev/answer/internal/schema"
	"github.com/answerdev/answer/internal/service/activity_common"
	"github.com/answerdev/answer/internal/service/comment_common"
	"github.com/answerdev/answer/internal/service/object_info"
	"github.com/answerdev/answer/internal/service/revision_common"
	"github.com/answerdev/answer/internal/service/tag_common"
	usercommon "github.com/answerdev/answer/internal/service/user_common"
	"github.com/answerdev/answer/pkg/converter"
	"github.com/segmentfault/pacman/log"
)

// ActivityRepo activity repository
type ActivityRepo interface {
	GetObjectAllActivity(ctx context.Context, objectID string, showVote bool) (activityList []*entity.Activity, err error)
}

// ActivityService activity service
type ActivityService struct {
	activityRepo          ActivityRepo
	userCommon            *usercommon.UserCommon
	activityCommonService *activity_common.ActivityCommon
	tagCommonService      *tag_common.TagCommonService
	objectInfoService     *object_info.ObjService
	commentCommonService  *comment_common.CommentCommonService
	revisionService       *revision_common.RevisionService
}

// NewActivityService new activity service
func NewActivityService(
	activityRepo ActivityRepo,
	userCommon *usercommon.UserCommon,
	activityCommonService *activity_common.ActivityCommon,
	tagCommonService *tag_common.TagCommonService,
	objectInfoService *object_info.ObjService,
	commentCommonService *comment_common.CommentCommonService,
	revisionService *revision_common.RevisionService,
) *ActivityService {
	return &ActivityService{
		objectInfoService:     objectInfoService,
		activityRepo:          activityRepo,
		userCommon:            userCommon,
		activityCommonService: activityCommonService,
		tagCommonService:      tagCommonService,
		commentCommonService:  commentCommonService,
		revisionService:       revisionService,
	}
}

// GetObjectTimeline get object timeline
func (as *ActivityService) GetObjectTimeline(ctx context.Context, req *schema.GetObjectTimelineReq) (
	resp *schema.GetObjectTimelineResp, err error) {
	resp = &schema.GetObjectTimelineResp{
		ObjectInfo: &schema.ActObjectInfo{},
		Timeline:   make([]*schema.ActObjectTimeline, 0),
	}

	objInfo, err := as.objectInfoService.GetInfo(ctx, req.ObjectID)
	if err != nil {
		return nil, err
	}
	resp.ObjectInfo.Title = objInfo.Title
	resp.ObjectInfo.ObjectType = objInfo.ObjectType
	resp.ObjectInfo.QuestionID = objInfo.QuestionID
	resp.ObjectInfo.AnswerID = objInfo.AnswerID

	activityList, err := as.activityRepo.GetObjectAllActivity(ctx, req.ObjectID, req.ShowVote)
	if err != nil {
		return nil, err
	}
	for _, act := range activityList {
		item := &schema.ActObjectTimeline{
			ActivityID: act.ID,
			RevisionID: converter.IntToString(act.RevisionID),
			CreatedAt:  act.CreatedAt.Unix(),
			Cancelled:  act.Cancelled == entity.ActivityCancelled,
			ObjectID:   act.ObjectID,
		}
		if item.Cancelled {
			item.CancelledAt = act.CancelledAt.Unix()
		}

		// database save activity type is number, change to activity type string is like "question.asked".
		// so we need to cut the front part of '.'
		item.ObjectType, item.ActivityType, _ = strings.Cut(config.ID2KeyMapping[act.ActivityType], ".")

		isHidden, formattedActivityType := formatActivity(item.ActivityType)
		if isHidden {
			continue
		}
		item.ActivityType = formattedActivityType

		// get user info
		userBasicInfo, exist, err := as.userCommon.GetUserBasicInfoByID(ctx, act.UserID)
		if err != nil {
			return nil, err
		}
		if exist {
			item.Username = userBasicInfo.Username
			item.UserDisplayName = userBasicInfo.DisplayName
		}

		if item.ObjectType == constant.CommentObjectType {
			comment, err := as.commentCommonService.GetComment(ctx, item.ObjectID)
			if err != nil {
				log.Error(err)
			} else {
				item.Comment = comment.ParsedText
			}
		}

		resp.Timeline = append(resp.Timeline, item)
	}
	return
}

// GetObjectTimelineDetail get object timeline
func (as *ActivityService) GetObjectTimelineDetail(ctx context.Context, req *schema.GetObjectTimelineDetailReq) (
	resp *schema.GetObjectTimelineDetailResp, err error) {
	resp = &schema.GetObjectTimelineDetailResp{}
	resp.OldRevision, err = as.getOneObjectDetail(ctx, req.OldRevisionID)
	if err != nil {
		return nil, err
	}
	resp.NewRevision, err = as.getOneObjectDetail(ctx, req.NewRevisionID)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetObjectTimelineDetail get object detail
func (as *ActivityService) getOneObjectDetail(ctx context.Context, revisionID string) (
	resp *schema.ObjectTimelineDetail, err error) {
	resp = &schema.ObjectTimelineDetail{Tags: make([]string, 0)}

	revision, err := as.revisionService.GetRevision(ctx, revisionID)
	if err != nil {
		return nil, err
	}
	objInfo, err := as.objectInfoService.GetInfo(ctx, revision.ObjectID)
	if err != nil {
		return nil, err
	}

	switch objInfo.ObjectType {
	case constant.QuestionObjectType:
		data := &entity.QuestionWithTagsRevision{}
		if err = json.Unmarshal([]byte(revision.Content), data); err != nil {
			log.Errorf("revision parsing error %s", err)
			return resp, nil
		}
		for _, tag := range data.Tags {
			resp.Tags = append(resp.Tags, tag.SlugName)
		}
		resp.Title = data.Title
		resp.OriginalText = data.OriginalText
	case constant.AnswerObjectType:
		data := &entity.Answer{}
		if err = json.Unmarshal([]byte(revision.Content), data); err != nil {
			log.Errorf("revision parsing error %s", err)
			return resp, nil
		}
		resp.Title = objInfo.Title // answer show question title
		resp.OriginalText = data.OriginalText
	case constant.TagObjectType:
		data := &entity.Tag{}
		if err = json.Unmarshal([]byte(revision.Content), data); err != nil {
			log.Errorf("revision parsing error %s", err)
			return resp, nil
		}
		resp.Title = data.SlugName
		resp.OriginalText = data.OriginalText
	default:
		log.Errorf("unknown object type %s", objInfo.ObjectType)
	}
	return resp, nil
}

func formatActivity(activityType string) (isHidden bool, formattedActivityType string) {
	if activityType == "voted_up" || activityType == "voted_down" || activityType == "accepted" {
		return true, ""
	}
	if activityType == "vote_up" {
		return false, "upvote"
	}
	if activityType == "vote_down" {
		return false, "downvote"
	}
	return false, activityType
}
